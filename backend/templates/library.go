package templates

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// LibraryIndexFile is the on-disk metadata file. It lives next to the
// individual template JSON files. If it is missing or corrupt,
// RebuildIndex can reconstruct it by scanning the directory.
const LibraryIndexFile = "_index.json"

// LibraryIndexVersion identifies the metadata schema. Bumped only on
// breaking changes to LibraryTemplateEntry.
const LibraryIndexVersion = 1

// LibraryTemplateEntry is one row in the index — fast metadata so the
// UI can list templates without parsing every JSON file. The Filename
// is relative to the library rootDir; never an absolute path.
type LibraryTemplateEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Filename    string   `json:"filename"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`

	InventoryItems int `json:"inventoryItems"`
	StorageItems   int `json:"storageItems"`
	Warnings       int `json:"warnings"`
}

// TemplateLibraryIndex is the persisted form of the directory metadata.
type TemplateLibraryIndex struct {
	Version int                    `json:"version"`
	Entries []LibraryTemplateEntry `json:"entries"`
}

// TemplateLibrary is the storage handle for a local templates directory.
// All public methods are safe for concurrent use via the embedded mutex.
type TemplateLibrary struct {
	rootDir string
	mu      sync.Mutex
}

// NewTemplateLibrary returns a handle bound to rootDir. The directory is
// not created here; callers wishing to use DefaultTemplateLibraryDir
// (which mkdirs) should call it first. Tests can pass an arbitrary
// rootDir (e.g. t.TempDir()) without going through DefaultTemplateLibraryDir.
func NewTemplateLibrary(rootDir string) *TemplateLibrary {
	return &TemplateLibrary{rootDir: rootDir}
}

// RootDir returns the directory this library reads from / writes to.
func (l *TemplateLibrary) RootDir() string {
	return l.rootDir
}

// DefaultTemplateLibraryDir returns the standard location for the
// per-user library: $UserConfigDir/EldenRing-SaveEditor/templates.
// The directory is created with 0700 permissions if missing.
func DefaultTemplateLibraryDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("DefaultTemplateLibraryDir: %w", err)
	}
	dir := filepath.Join(configDir, "EldenRing-SaveEditor", "templates")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("DefaultTemplateLibraryDir: %w", err)
	}
	return dir, nil
}

// SaveTemplate validates, assigns an ID, writes the file atomically, and
// updates the index. The returned entry mirrors what would land in the
// index file. The template's Metadata.Name / Description / Tags are the
// authoritative source for the entry's user-facing fields.
func (l *TemplateLibrary) SaveTemplate(tpl *BuildTemplate) (LibraryTemplateEntry, error) {
	if tpl == nil {
		return LibraryTemplateEntry{}, fmt.Errorf("SaveTemplate: nil template")
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		return LibraryTemplateEntry{}, fmt.Errorf("SaveTemplate: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(l.rootDir, 0700); err != nil {
		return LibraryTemplateEntry{}, fmt.Errorf("SaveTemplate: mkdir: %w", err)
	}

	id, err := generateTemplateID()
	if err != nil {
		return LibraryTemplateEntry{}, err
	}

	name := ""
	desc := ""
	var tags []string
	if tpl.Metadata != nil {
		name = tpl.Metadata.Name
		desc = tpl.Metadata.Description
		tags = append(tags, tpl.Metadata.Tags...)
	}

	filename, err := l.uniqueFilenameLocked(name, id)
	if err != nil {
		return LibraryTemplateEntry{}, err
	}

	nowUTC := time.Now().UTC().Format(time.RFC3339Nano)
	entry := LibraryTemplateEntry{
		ID:             id,
		Name:           name,
		Description:    desc,
		Tags:           tags,
		Filename:       filename,
		CreatedAt:      nowUTC,
		UpdatedAt:      nowUTC,
		InventoryItems: countInventoryItems(tpl),
		StorageItems:   countStorageItems(tpl),
	}

	data, err := json.MarshalIndent(tpl, "", "  ")
	if err != nil {
		return LibraryTemplateEntry{}, fmt.Errorf("SaveTemplate: marshal: %w", err)
	}
	if err := atomicWriteFile(filepath.Join(l.rootDir, filename), data, 0644); err != nil {
		return LibraryTemplateEntry{}, err
	}

	idx, _ := l.readIndexLocked()
	idx.Entries = append(idx.Entries, entry)
	if err := l.writeIndexLocked(idx); err != nil {
		return LibraryTemplateEntry{}, err
	}
	return entry, nil
}

// ListTemplates returns the current index entries, sorted by UpdatedAt
// descending (newest first) — the order most useful for a UI list. The
// returned slice is a copy; callers may mutate it freely.
func (l *TemplateLibrary) ListTemplates() ([]LibraryTemplateEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	idx, err := l.readIndexLocked()
	if err != nil {
		return nil, err
	}
	out := append([]LibraryTemplateEntry(nil), idx.Entries...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out, nil
}

// LoadTemplate reads and validates a template by ID.
func (l *TemplateLibrary) LoadTemplate(id string) (*BuildTemplate, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, err := l.findEntryLocked(id)
	if err != nil {
		return nil, err
	}
	tpl, err := readAndValidateTemplate(filepath.Join(l.rootDir, entry.Filename))
	if err != nil {
		return nil, fmt.Errorf("LoadTemplate %s: %w", id, err)
	}
	return tpl, nil
}

// DeleteTemplate removes the file and the index entry. Missing file is
// not an error (the entry is removed either way) — this keeps the index
// in sync even if the user manually deleted the file.
func (l *TemplateLibrary) DeleteTemplate(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	idx, err := l.readIndexLocked()
	if err != nil {
		return err
	}
	for i, e := range idx.Entries {
		if e.ID == id {
			idx.Entries = append(idx.Entries[:i], idx.Entries[i+1:]...)
			path := filepath.Join(l.rootDir, e.Filename)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("DeleteTemplate: %w", err)
			}
			return l.writeIndexLocked(idx)
		}
	}
	return fmt.Errorf("DeleteTemplate: template %q not found", id)
}

// RenameTemplate updates Name / Description / Tags inside both the
// template file's Metadata and the index entry. UpdatedAt is bumped.
// Returns the updated entry.
func (l *TemplateLibrary) RenameTemplate(id, name, description string, tags []string) (LibraryTemplateEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	idx, err := l.readIndexLocked()
	if err != nil {
		return LibraryTemplateEntry{}, err
	}
	pos := -1
	for i, e := range idx.Entries {
		if e.ID == id {
			pos = i
			break
		}
	}
	if pos < 0 {
		return LibraryTemplateEntry{}, fmt.Errorf("RenameTemplate: template %q not found", id)
	}
	entry := idx.Entries[pos]

	tplPath := filepath.Join(l.rootDir, entry.Filename)
	tpl, err := readAndValidateTemplate(tplPath)
	if err != nil {
		return LibraryTemplateEntry{}, fmt.Errorf("RenameTemplate: %w", err)
	}
	if tpl.Metadata == nil {
		tpl.Metadata = &TemplateMetadata{}
	}
	tpl.Metadata.Name = name
	tpl.Metadata.Description = description
	tpl.Metadata.Tags = append([]string(nil), tags...)

	data, err := json.MarshalIndent(tpl, "", "  ")
	if err != nil {
		return LibraryTemplateEntry{}, fmt.Errorf("RenameTemplate: marshal: %w", err)
	}
	if err := atomicWriteFile(tplPath, data, 0644); err != nil {
		return LibraryTemplateEntry{}, err
	}

	entry.Name = name
	entry.Description = description
	entry.Tags = append([]string(nil), tags...)
	entry.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	idx.Entries[pos] = entry
	if err := l.writeIndexLocked(idx); err != nil {
		return LibraryTemplateEntry{}, err
	}
	return entry, nil
}

// ExportTemplateToFile copies a stored template to an arbitrary path.
// The on-disk format is identical to the per-library file; this exists
// so a user can hand a shareable copy to someone else without exposing
// the library directory.
func (l *TemplateLibrary) ExportTemplateToFile(id, path string) error {
	if path == "" {
		return fmt.Errorf("ExportTemplateToFile: empty destination path")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, err := l.findEntryLocked(id)
	if err != nil {
		return err
	}
	src := filepath.Join(l.rootDir, entry.Filename)
	tpl, err := readAndValidateTemplate(src)
	if err != nil {
		return fmt.Errorf("ExportTemplateToFile: %w", err)
	}
	data, err := json.MarshalIndent(tpl, "", "  ")
	if err != nil {
		return fmt.Errorf("ExportTemplateToFile: marshal: %w", err)
	}
	if err := atomicWriteFile(path, data, 0644); err != nil {
		return err
	}
	return nil
}

// RebuildIndex scans the rootDir for valid template JSON files and
// regenerates _index.json from scratch. Files that fail to parse or
// validate are skipped — they remain on disk but do not appear in the
// new index. Existing IDs and timestamps are preserved when possible
// (looked up in the old index by filename).
func (l *TemplateLibrary) RebuildIndex() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rebuildIndexLocked()
}

func (l *TemplateLibrary) rebuildIndexLocked() error {
	if err := os.MkdirAll(l.rootDir, 0700); err != nil {
		return fmt.Errorf("RebuildIndex: mkdir: %w", err)
	}
	prev := TemplateLibraryIndex{Version: LibraryIndexVersion}
	if data, err := os.ReadFile(filepath.Join(l.rootDir, LibraryIndexFile)); err == nil {
		_ = json.Unmarshal(data, &prev)
	}
	prevByFilename := make(map[string]LibraryTemplateEntry, len(prev.Entries))
	for _, e := range prev.Entries {
		prevByFilename[e.Filename] = e
	}

	entries, err := os.ReadDir(l.rootDir)
	if err != nil {
		return fmt.Errorf("RebuildIndex: %w", err)
	}
	var rebuilt []LibraryTemplateEntry
	for _, ent := range entries {
		if ent.IsDir() || ent.Name() == LibraryIndexFile {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(ent.Name()), ".json") {
			continue
		}
		tpl, err := readAndValidateTemplate(filepath.Join(l.rootDir, ent.Name()))
		if err != nil {
			continue
		}
		entry := buildEntryFromTemplate(tpl, ent.Name(), prevByFilename[ent.Name()])
		rebuilt = append(rebuilt, entry)
	}
	idx := TemplateLibraryIndex{Version: LibraryIndexVersion, Entries: rebuilt}
	return l.writeIndexLocked(idx)
}

// ─── internal helpers ───────────────────────────────────────────────────

func (l *TemplateLibrary) findEntryLocked(id string) (LibraryTemplateEntry, error) {
	idx, err := l.readIndexLocked()
	if err != nil {
		return LibraryTemplateEntry{}, err
	}
	for _, e := range idx.Entries {
		if e.ID == id {
			return e, nil
		}
	}
	return LibraryTemplateEntry{}, fmt.Errorf("template %q not found", id)
}

// readIndexLocked loads the on-disk index. A missing index file is not
// an error and returns an empty index — this lets SaveTemplate
// initialise a fresh library on first use without auto-discovering
// files it is about to write (which would race the save). A corrupt
// (present but unparseable) index *does* trigger a rebuild from the
// directory contents, since at that point we cannot proceed otherwise.
// Users who manually drop JSON files into the directory must invoke
// RebuildIndex explicitly.
func (l *TemplateLibrary) readIndexLocked() (TemplateLibraryIndex, error) {
	path := filepath.Join(l.rootDir, LibraryIndexFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return TemplateLibraryIndex{Version: LibraryIndexVersion, Entries: []LibraryTemplateEntry{}}, nil
		}
		return TemplateLibraryIndex{}, fmt.Errorf("readIndex: %w", err)
	}
	var idx TemplateLibraryIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		if rebuildErr := l.rebuildIndexLocked(); rebuildErr != nil {
			return TemplateLibraryIndex{}, fmt.Errorf("readIndex: corrupt index and rebuild failed: %w", rebuildErr)
		}
		data, err = os.ReadFile(path)
		if err != nil {
			return TemplateLibraryIndex{}, fmt.Errorf("readIndex: post-rebuild read: %w", err)
		}
		idx = TemplateLibraryIndex{}
		if err := json.Unmarshal(data, &idx); err != nil {
			return TemplateLibraryIndex{}, fmt.Errorf("readIndex: post-rebuild unmarshal: %w", err)
		}
	}
	if idx.Version == 0 {
		idx.Version = LibraryIndexVersion
	}
	return idx, nil
}

func (l *TemplateLibrary) writeIndexLocked(idx TemplateLibraryIndex) error {
	if idx.Version == 0 {
		idx.Version = LibraryIndexVersion
	}
	if idx.Entries == nil {
		idx.Entries = []LibraryTemplateEntry{}
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("writeIndex: %w", err)
	}
	return atomicWriteFile(filepath.Join(l.rootDir, LibraryIndexFile), data, 0644)
}

// uniqueFilenameLocked picks a filename derived from the template's
// display name (sanitised) plus a short tail of the ID. Collisions are
// resolved by appending an incrementing numeric suffix before .json.
func (l *TemplateLibrary) uniqueFilenameLocked(name, id string) (string, error) {
	stem := sanitizeFilenameStem(name)
	if stem == "" {
		stem = "template"
	}
	idTail := id
	if len(idTail) > 8 {
		idTail = idTail[len(idTail)-8:]
	}
	base := fmt.Sprintf("%s-%s", stem, idTail)
	candidate := base + ".json"
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(l.rootDir, candidate)); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("uniqueFilename: %w", err)
		}
		candidate = fmt.Sprintf("%s-%d.json", base, i)
		if i > 1000 {
			return "", fmt.Errorf("uniqueFilename: too many collisions for %q", base)
		}
	}
}

// filenameStemSanitizer strips characters that are unsafe across the
// three desktop OSes. We also clamp the length so a 200-char user name
// does not produce a 200-char filename.
var filenameStemSanitizer = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func sanitizeFilenameStem(name string) string {
	stem := filenameStemSanitizer.ReplaceAllString(name, "-")
	stem = strings.Trim(stem, "-_.")
	if len(stem) > 60 {
		stem = strings.Trim(stem[:60], "-_.")
	}
	return stem
}

// generateTemplateID returns a sortable, monotonic-ish id of the form
// "YYYYMMDDTHHMMSS-HHHHHHHH" where the random tail breaks ties when two
// SaveTemplate calls land in the same second.
func generateTemplateID() (string, error) {
	var b [4]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return "", fmt.Errorf("generateTemplateID: %w", err)
	}
	ts := time.Now().UTC().Format("20060102T150405")
	return fmt.Sprintf("%s-%s", ts, hex.EncodeToString(b[:])), nil
}

// atomicWriteFile writes data to a sibling temp file then renames into
// place. On error the temp file is removed so the caller never sees a
// partial result. The directory's existing fsync semantics are good
// enough for this app — we are not journaling and a crash mid-write
// leaves the prior file (if any) intact.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".saveforge-tmp-*")
	if err != nil {
		return fmt.Errorf("atomicWrite: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("atomicWrite: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("atomicWrite: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("atomicWrite: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		cleanup()
		return fmt.Errorf("atomicWrite: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("atomicWrite: %w", err)
	}
	return nil
}

func readAndValidateTemplate(path string) (*BuildTemplate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tpl BuildTemplate
	if err := json.Unmarshal(data, &tpl); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if err := ValidateBuildTemplate(&tpl); err != nil {
		return nil, err
	}
	return &tpl, nil
}

// buildEntryFromTemplate constructs an index row from a parsed template.
// When a previous entry exists (filename match during rebuild), its ID
// and CreatedAt are preserved so a rebuild does not look like a "new
// template" to UIs that key off ID.
func buildEntryFromTemplate(tpl *BuildTemplate, filename string, prev LibraryTemplateEntry) LibraryTemplateEntry {
	nowUTC := time.Now().UTC().Format(time.RFC3339Nano)
	entry := LibraryTemplateEntry{
		Filename:       filename,
		InventoryItems: countInventoryItems(tpl),
		StorageItems:   countStorageItems(tpl),
		UpdatedAt:      nowUTC,
	}
	if tpl.Metadata != nil {
		entry.Name = tpl.Metadata.Name
		entry.Description = tpl.Metadata.Description
		entry.Tags = append([]string(nil), tpl.Metadata.Tags...)
	}
	if prev.ID != "" {
		entry.ID = prev.ID
		entry.CreatedAt = prev.CreatedAt
		if prev.UpdatedAt != "" {
			entry.UpdatedAt = prev.UpdatedAt
		}
		entry.Warnings = prev.Warnings
	} else {
		// Recovered file we have not indexed before — mint a fresh ID
		// but keep the filename as the stable handle on disk.
		id, err := generateTemplateID()
		if err == nil {
			entry.ID = id
		}
		entry.CreatedAt = nowUTC
	}
	if tpl.CreatedAt != "" && entry.CreatedAt == "" {
		entry.CreatedAt = tpl.CreatedAt
	}
	return entry
}

func countInventoryItems(tpl *BuildTemplate) int {
	if tpl == nil || tpl.Sections.InventoryWorkspace == nil {
		return 0
	}
	return len(tpl.Sections.InventoryWorkspace.InventoryItems)
}

func countStorageItems(tpl *BuildTemplate) int {
	if tpl == nil || tpl.Sections.InventoryWorkspace == nil {
		return 0
	}
	return len(tpl.Sections.InventoryWorkspace.StorageItems)
}

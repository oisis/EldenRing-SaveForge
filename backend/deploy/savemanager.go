package deploy

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"
)

// BackupMeta is stored as a .json sidecar next to each .bak file.
type BackupMeta struct {
	MD5       string    `json:"md5"`
	Tags      []string  `json:"tags"`
	Desc      string    `json:"desc"`
	CreatedAt time.Time `json:"created_at"`
}

// SaveBackupEntry is returned to the frontend for each .bak file found on a target.
// Timestamp is an RFC3339 string so Wails can represent it as a plain TypeScript string.
type SaveBackupEntry struct {
	Name      string   `json:"name"`
	Timestamp string   `json:"timestamp"`
	Size      int64    `json:"size"`
	MD5       string   `json:"md5"`
	Tags      []string `json:"tags"`
	Desc      string   `json:"desc"`
	IsActive  bool     `json:"isActive"`
}

// metaPath returns the .json sidecar path for a given .bak path.
func metaPath(bakPath string) string {
	return bakPath + ".json"
}

// readMeta deserialises a BackupMeta from raw JSON bytes.
// Tolerates missing or malformed JSON — returns zero value with empty Tags slice.
func readMeta(data []byte) BackupMeta {
	var m BackupMeta
	json.Unmarshal(data, &m) //nolint:errcheck — partial/missing JSON is fine
	if m.Tags == nil {
		m.Tags = []string{}
	}
	return m
}

// marshalMeta serialises a BackupMeta to indented JSON bytes.
func marshalMeta(m BackupMeta) []byte {
	if m.Tags == nil {
		m.Tags = []string{}
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	return data
}

// computeMD5 returns the hex md5 of data.
func computeMD5(data []byte) string {
	sum := md5.Sum(data)
	return fmt.Sprintf("%x", sum)
}

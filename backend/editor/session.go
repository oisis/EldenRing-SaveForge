package editor

import (
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// ErrSessionClosed is returned by InventoryEditSession.Acquire when the
// session has been discarded (Close) before the caller could lock it.
// Callers map this to the same wire-level message as a missing session
// so the frontend's existing self-heal path treats both identically.
var ErrSessionClosed = errors.New("inventory edit session is closed")

// InventoryEditSession owns a single workspace snapshot for a character.
//
// The session is held in-memory on App and never persisted to disk.
// BaselineEditableHandles captures the (record UID → container) state at
// Start time and is consumed by ApplyWorkspaceSave to detect transfers
// and removals against the original load. It is keyed by EditableItem.UID
// rather than OriginalHandle: a handle alone does not identify a physical
// record — talisman handles (and any other item-derived handle) are
// legitimately shared by multiple records in the same container or split
// across inventory/storage, and a handle-keyed map can only remember one
// of them. UID (container+slot+handle) disambiguates every such record.
// Phase 3B regenerates this after every successful save so subsequent
// edits diff against the freshly-reparsed state rather than the original
// load.
//
// Concurrency contract:
//   - Lifecycle (insert/replace/delete in the App registry) is governed
//     by a registry-scoped mutex owned by the caller (package main).
//   - Mutations and reads of Workspace, BaselineEditableHandles and
//     BaseRevision MUST run under Acquire/Unlock so the multi-step
//     SaveInventoryWorkspaceChanges flow cannot race with concurrent
//     mutators or readers. Acquire also enforces the closed-after-Discard
//     contract: callers that arrive after Close get ErrSessionClosed
//     instead of touching an orphaned struct.
type InventoryEditSession struct {
	ID                      string                     `json:"id"`
	CharacterIndex          int                        `json:"characterIndex"`
	CreatedAt               time.Time                  `json:"createdAt"`
	BaseRevision            string                     `json:"baseRevision"`
	Workspace               InventoryWorkspaceSnapshot `json:"workspace"`
	BaselineEditableHandles map[string]ContainerKind   `json:"-"`

	// mu serializes every read/write of the mutable session state
	// (Workspace, BaselineEditableHandles, BaseRevision, closed). The
	// SaveInventoryWorkspaceChanges path holds it across rollback +
	// snapshot rebuild + baseline regeneration so no peer ever sees a
	// half-rewritten workspace or a baseline map in the middle of
	// initialisation.
	mu sync.Mutex
	// closed is set under mu when Discard / clearAllEditSessions wants
	// to invalidate the session. Acquire checks it after taking the
	// lock so a mutator that won the lock race against Discard still
	// fails fast with ErrSessionClosed instead of mutating an orphan.
	closed bool
}

// Acquire locks the session for exclusive use. It returns ErrSessionClosed
// when the session has already been Close()-d, in which case the lock is
// NOT held and Unlock must NOT be called. On nil error the caller owns the
// lock and must release it with Unlock — typically via defer.
func (s *InventoryEditSession) Acquire() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrSessionClosed
	}
	return nil
}

// Unlock releases the session lock acquired by Acquire. It is unsafe to
// call without a prior successful Acquire (matches sync.Mutex semantics).
func (s *InventoryEditSession) Unlock() {
	s.mu.Unlock()
}

// Close marks the session as invalidated. The caller MUST hold the
// session lock (acquired via Acquire) — Close itself does not lock so
// the discard path can mark-and-release in one critical section.
//
// Subsequent Acquire calls return ErrSessionClosed. Operations that
// already passed Acquire continue safely against the (orphan) struct
// because they hold the lock; once they Unlock the struct is unreachable
// from the registry and becomes garbage.
func (s *InventoryEditSession) Close() {
	s.closed = true
}

// IsClosed reports whether the session has been Close()-d. The check
// briefly takes the lock so the result reflects a committed state
// rather than a half-written one — fine for tests / diagnostics, not a
// substitute for the Acquire failure path on the mutator hot path.
func (s *InventoryEditSession) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// NewSessionID returns a short random hex token used as session identifier.
//
// 8 random bytes give a 128-bit space — far more than enough since App
// limits one active session per character (≤ 10 concurrent sessions).
// The crypto/rand failure path falls back to a nanosecond timestamp so
// startup never blocks on a missing entropy source.
func NewSessionID() string {
	var buf [8]byte
	if _, err := crand.Read(buf[:]); err != nil {
		return fmt.Sprintf("ses-%016x", time.Now().UnixNano())
	}
	return "ses-" + hex.EncodeToString(buf[:])
}

// ComputeBaseRevision derives a content marker from a few stable slot
// fields. It exists so the frontend / save path can detect if the
// underlying slot has changed between Start and a future Save (e.g. the
// user reloaded the save file). It is not a security hash.
func ComputeBaseRevision(slot *core.SaveSlot) string {
	h := sha256.New()
	fmt.Fprintf(h, "v=%d ga=%d magic=%d", slot.Version, len(slot.GaItems), slot.MagicOffset)
	if len(slot.Data) >= 1024 {
		h.Write(slot.Data[:1024])
	} else {
		h.Write(slot.Data)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// StartSession builds a fresh InventoryEditSession from the given slot.
// The slot is not mutated. Returns the populated session, ready to be
// stored in App's session map.
func StartSession(slot *core.SaveSlot, charIdx int) (*InventoryEditSession, error) {
	if slot == nil {
		return nil, fmt.Errorf("StartSession: nil slot")
	}
	id := NewSessionID()
	snap, err := BuildSnapshot(slot, id, charIdx)
	if err != nil {
		return nil, fmt.Errorf("StartSession: %w", err)
	}
	// Run validation immediately so the snapshot ships with a current
	// report — callers that only Start without Validate still see issues.
	snap.Validation = Validate(snap)
	baseline := make(map[string]ContainerKind, len(snap.InventoryItems)+len(snap.StorageItems))
	for _, it := range snap.InventoryItems {
		if it.Source == ItemSourceOriginal && it.OriginalHandle != 0 {
			baseline[it.UID] = ContainerInventory
		}
	}
	for _, it := range snap.StorageItems {
		if it.Source == ItemSourceOriginal && it.OriginalHandle != 0 {
			baseline[it.UID] = ContainerStorage
		}
	}
	sess := &InventoryEditSession{
		ID:                      id,
		CharacterIndex:          charIdx,
		CreatedAt:               time.Now().UTC(),
		BaseRevision:            ComputeBaseRevision(slot),
		Workspace:               snap,
		BaselineEditableHandles: baseline,
	}
	return sess, nil
}

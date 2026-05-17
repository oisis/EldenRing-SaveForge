package editor

import (
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// InventoryEditSession owns a single workspace snapshot for a character.
//
// Phase 1 sessions are immutable after Start — mutating APIs land in
// later phases. The session is held in-memory on App and never persisted
// to disk.
type InventoryEditSession struct {
	ID             string                     `json:"id"`
	CharacterIndex int                        `json:"characterIndex"`
	CreatedAt      time.Time                  `json:"createdAt"`
	BaseRevision   string                     `json:"baseRevision"`
	Workspace      InventoryWorkspaceSnapshot `json:"workspace"`
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
	sess := &InventoryEditSession{
		ID:             id,
		CharacterIndex: charIdx,
		CreatedAt:      time.Now().UTC(),
		BaseRevision:   ComputeBaseRevision(slot),
		Workspace:      snap,
	}
	return sess, nil
}

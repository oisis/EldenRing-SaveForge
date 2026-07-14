package main

import (
	"encoding/json"
	"testing"
)

func TestGaItemRepackAPIJSONContracts(t *testing.T) {
	before := GaItemCapacity{PhysicalEmpty: 3, CursorRoom: 1, Usable: 1}
	after := GaItemCapacity{PhysicalEmpty: 3, CursorRoom: 3, Usable: 3}
	failure := &GaItemRepackFailure{Stage: "preflight", Code: "preflight_refusal", Message: "The slot no longer passes the GaItem repack safety checks."}

	tests := []struct {
		name    string
		value   any
		present []string
		absent  []string
	}{
		{
			name: "ready analysis",
			value: GaItemRepackAnalysis{
				Outcome: "ready", CharacterIndex: 2, AnalysisToken: "token", Before: before,
				ProjectedAfter: &after, Recovered: 2, NonEmptyRecords: 5117, Blockers: []GaItemRepackBlocker{},
			},
			present: []string{"outcome", "characterIndex", "analysisToken", "before", "projectedAfter", "recovered", "nonEmptyRecords", "blockers"},
			absent:  []string{"failure"},
		},
		{
			name: "refusal analysis",
			value: GaItemRepackAnalysis{
				Outcome: "refusal", CharacterIndex: 2, Before: before,
				Blockers: []GaItemRepackBlocker{{Code: "duplicate_handle", Message: "Two records share a handle."}},
			},
			present: []string{"outcome", "characterIndex", "before", "recovered", "nonEmptyRecords", "blockers"},
			absent:  []string{"analysisToken", "projectedAfter", "failure"},
		},
		{
			name: "success execution",
			value: GaItemRepackExecutionResult{
				Outcome: "success", CharacterIndex: 2, Before: before, After: &after, Recovered: 2,
			},
			present: []string{"outcome", "characterIndex", "before", "after", "recovered"},
			absent:  []string{"failure", "rollback"},
		},
		{
			name: "rolled back execution",
			value: GaItemRepackExecutionResult{
				Outcome: "rolled_back", CharacterIndex: 2, Before: before, Failure: failure,
				Rollback: &GaItemRepackRollback{Attempted: true, Complete: true, Mode: "discard_candidate"},
			},
			present: []string{"outcome", "characterIndex", "before", "recovered", "failure", "rollback"},
			absent:  []string{"after"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := jsonObject(t, tc.value)
			for _, key := range tc.present {
				if _, ok := payload[key]; !ok {
					t.Errorf("JSON key %q missing from %v", key, payload)
				}
			}
			for _, key := range tc.absent {
				if _, ok := payload[key]; ok {
					t.Errorf("JSON key %q unexpectedly present in %v", key, payload)
				}
			}
		})
	}
}

func TestAddResultJSONIncludesCTAFieldsOnlyWhenProvided(t *testing.T) {
	base := AddResult{Added: 0, Requested: 1, Trimmed: []SkippedAdd{}, SkippedExisting: []SkippedAdd{}, CapHit: "gaitem_full"}
	withoutCTA := jsonObject(t, base)
	for _, key := range []string{"gaItemCapacity", "gaItemRepackCTA"} {
		if _, ok := withoutCTA[key]; ok {
			t.Fatalf("JSON key %q unexpectedly present without CTA data", key)
		}
	}

	capacity := GaItemCapacity{PhysicalEmpty: 3, CursorRoom: 1, Usable: 1}
	withCTA := base
	withCTA.GaItemCapacity = &capacity
	withCTA.GaItemRepackCTA = &GaItemRepackCTA{Eligible: true, Recovered: 2}
	payload := jsonObject(t, withCTA)
	if got := payload["gaItemCapacity"]; got == nil {
		t.Fatal("gaItemCapacity missing")
	}
	if got := payload["gaItemRepackCTA"]; got == nil {
		t.Fatal("gaItemRepackCTA missing")
	}
}

func jsonObject(t *testing.T, value any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return payload
}

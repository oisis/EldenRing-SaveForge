package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

func TestSetNetworkParamsLogsDurableFailureLifecycle(t *testing.T) {
	app := withJournal(NewApp())
	if err := app.SetNetworkParams(core.NetworkParamDefaults()); err == nil {
		t.Fatal("SetNetworkParams succeeded without a save")
	}
	records := app.journal.Tail()
	requested := operationEvent(t, records, "network_params_requested")
	finished := operationEvent(t, records, "network_params_finished")
	if requested.Seq >= finished.Seq {
		t.Errorf("requested seq %d must precede finished seq %d", requested.Seq, finished.Seq)
	}
	if got := operationField(requested, "action"); got != "network_params_apply" {
		t.Errorf("action = %q, want network_params_apply", got)
	}
	if got := finished.Level; got != levelError {
		t.Errorf("level = %q, want error", got)
	}
	if got := operationField(finished, "stage"); got != "no_active_save" {
		t.Errorf("stage = %q, want no_active_save", got)
	}
}

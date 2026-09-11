package main

import (
	"reflect"
	"testing"
)

func TestAppFacadePreservesWailsMethodSet(t *testing.T) {
	appType := reflect.TypeOf(NewApp())
	const expectedMethods = 183
	if appType.NumMethod() != expectedMethods {
		t.Fatalf("App exported method count = %d, want %d", appType.NumMethod(), expectedMethods)
	}

	criticalMethods := []string{
		"SelectAndOpenSave",
		"GetCharacter",
		"SaveCharacter",
		"GetEquipmentSnapshot",
		"SaveEquipment",
		"StartInventoryEditSession",
		"SaveInventoryWorkspaceChanges",
		"RunDiagnosticsAllLoaded",
		"ApplyBuildTemplateV2ToCharacterJSON",
		"WriteSave",
	}
	for _, methodName := range criticalMethods {
		if _, ok := appType.MethodByName(methodName); !ok {
			t.Errorf("App facade does not expose %s", methodName)
		}
	}
}

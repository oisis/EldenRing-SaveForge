import { editor } from '../../wailsjs/go/models';

// Helpers that build the WeaponPatch DTO for the four user actions the
// WeaponEditModal exposes in workspace mode. Kept separate so the action
// → patch mapping is unit-testable without mounting the full modal
// (which pulls in several Wails calls at mount time).
//
// Contract for the backend (mirrored from editor.WeaponPatch):
//   - exactly the field set the user is changing is marked with its
//     paired set* / clearAoW boolean
//   - infusionName="" represents the Standard / no-infusion state
//   - clearAoW supersedes setAoWItemID; the modal never sets both

export function upgradePatch(level: number): editor.WeaponPatch {
    return editor.WeaponPatch.createFrom({ setUpgrade: true, upgrade: level });
}

export function infusionPatch(infusionName: string): editor.WeaponPatch {
    // The "Standard" infusion is encoded as an empty string by the
    // backend, so normalize here too. Callers can pass either form.
    const stored = infusionName === 'Standard' ? '' : infusionName;
    return editor.WeaponPatch.createFrom({ setInfusionName: true, infusionName: stored });
}

export function aowAssignPatch(aowItemID: number): editor.WeaponPatch {
    return editor.WeaponPatch.createFrom({ setAoWItemID: true, aowItemID });
}

export function aowClearPatch(): editor.WeaponPatch {
    return editor.WeaponPatch.createFrom({ clearAoW: true });
}

// aowApplyPatch matches WeaponEditModal.applyAoW behavior: id===0 clears,
// any other id assigns. Centralizes the conditional so the modal and
// tests share a single source of truth.
export function aowApplyPatch(aowItemID: number): editor.WeaponPatch {
    return aowItemID === 0 ? aowClearPatch() : aowAssignPatch(aowItemID);
}

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
    AddInventoryWorkspaceItem,
    DiscardInventoryEditSession,
    MoveInventoryWorkspaceItem,
    RemoveInventoryWorkspaceItem,
    SaveInventoryWorkspaceChanges,
    StartInventoryEditSession,
    TransferInventoryWorkspaceItem,
    UpdateInventoryWorkspaceWeapon,
    ValidateInventoryWorkspace,
} from '../../wailsjs/go/main/App';
import { editor } from '../../wailsjs/go/models';

export type ContainerKind = 'inventory' | 'storage';

export interface UseInventoryWorkspaceResult {
    sessionID: string;
    characterIndex: number | null;
    inventoryItems: editor.EditableItem[];
    storageItems: editor.EditableItem[];
    validation: editor.WorkspaceValidationReport | null;
    dirty: boolean;
    loading: boolean;
    saving: boolean;
    lastError: string | null;
    clearError: () => void;
    start: (charIndex: number) => Promise<editor.InventoryWorkspaceSnapshot | null>;
    refresh: () => Promise<void>;
    moveItem: (uid: string, target: ContainerKind, targetPosition: number) => Promise<void>;
    transferItem: (uid: string, target: ContainerKind) => Promise<void>;
    addItem: (spec: editor.AddItemSpec, target: ContainerKind, targetPosition: number) => Promise<editor.EditableItem | null>;
    removeItem: (uid: string) => Promise<void>;
    updateWeapon: (uid: string, patch: editor.WeaponPatch) => Promise<editor.EditableItem | null>;
    save: () => Promise<editor.InventoryWorkspaceSnapshot | null>;
    discard: () => Promise<void>;
    // replaceSnapshot pushes a workspace produced outside the hook
    // (e.g. an Apply Template call) into local state. Phase D uses this
    // so the Apply flow can swap in the post-apply snapshot without
    // re-fetching from the backend.
    replaceSnapshot: (snap: editor.InventoryWorkspaceSnapshot) => void;
}

function snapshotError(label: string, err: unknown): string {
    const msg = err instanceof Error ? err.message : String(err);
    return msg ? `${label}: ${msg}` : label;
}

function findByUID(items: editor.EditableItem[], uid: string): editor.EditableItem | null {
    return items.find(it => it.uid === uid) ?? null;
}

export function useInventoryWorkspace(): UseInventoryWorkspaceResult {
    const [snapshot, setSnapshot] = useState<editor.InventoryWorkspaceSnapshot | null>(null);
    const [loading, setLoading] = useState(false);
    const [saving, setSaving] = useState(false);
    const [lastError, setLastError] = useState<string | null>(null);

    // Active session id, surfaced as plain state so callers can react to it.
    // Tracked in a ref to avoid stale-closure issues inside async callbacks.
    const sessionIDRef = useRef<string>('');
    const characterIndexRef = useRef<number | null>(null);

    const applySnapshot = useCallback((snap: editor.InventoryWorkspaceSnapshot | null) => {
        setSnapshot(snap);
        if (snap) {
            sessionIDRef.current = snap.sessionID;
            characterIndexRef.current = snap.characterIndex;
        } else {
            sessionIDRef.current = '';
            characterIndexRef.current = null;
        }
    }, []);

    const handleError = useCallback((label: string, err: unknown) => {
        const message = snapshotError(label, err);
        setLastError(message);
        return message;
    }, []);

    const start = useCallback(async (charIndex: number) => {
        setLoading(true);
        setLastError(null);
        try {
            const snap = await StartInventoryEditSession(charIndex);
            applySnapshot(snap);
            return snap;
        } catch (err) {
            handleError('Failed to start inventory session', err);
            applySnapshot(null);
            return null;
        } finally {
            setLoading(false);
        }
    }, [applySnapshot, handleError]);

    const requireSession = useCallback((label: string): string | null => {
        const id = sessionIDRef.current;
        if (!id) {
            handleError(label, new Error('no active session'));
            return null;
        }
        return id;
    }, [handleError]);

    const refresh = useCallback(async () => {
        const id = sessionIDRef.current;
        if (!id) return;
        try {
            const report = await ValidateInventoryWorkspace(id);
            setSnapshot(prev => (prev ? editor.InventoryWorkspaceSnapshot.createFrom({ ...prev, validation: report }) : prev));
        } catch (err) {
            handleError('Validation refresh failed', err);
        }
    }, [handleError]);

    const moveItem = useCallback(async (uid: string, target: ContainerKind, targetPosition: number) => {
        const id = requireSession('Move failed');
        if (!id) return;
        try {
            const next = await MoveInventoryWorkspaceItem(id, uid, target, targetPosition);
            applySnapshot(next);
        } catch (err) {
            handleError('Move failed', err);
        }
    }, [applySnapshot, handleError, requireSession]);

    const transferItem = useCallback(async (uid: string, target: ContainerKind) => {
        const id = requireSession('Transfer failed');
        if (!id) return;
        try {
            const next = await TransferInventoryWorkspaceItem(id, uid, target);
            applySnapshot(next);
        } catch (err) {
            handleError('Transfer failed', err);
        }
    }, [applySnapshot, handleError, requireSession]);

    const addItem = useCallback(async (spec: editor.AddItemSpec, target: ContainerKind, targetPosition: number) => {
        const id = requireSession('Add failed');
        if (!id) return null;
        try {
            const beforeUIDs = new Set((snapshot?.inventoryItems ?? []).concat(snapshot?.storageItems ?? []).map(it => it.uid));
            // Backend AddItem clamps negative targetPosition to 0 (prepend), so
            // translate "-1 means append" at the boundary by computing the
            // current destination length from the active snapshot.
            const dstLen = target === 'inventory'
                ? (snapshot?.inventoryItems.length ?? 0)
                : (snapshot?.storageItems.length ?? 0);
            const effectivePos = targetPosition < 0 ? dstLen : targetPosition;
            const next = await AddInventoryWorkspaceItem(id, spec, target, effectivePos);
            applySnapshot(next);
            const pool = target === 'inventory' ? next.inventoryItems : next.storageItems;
            const newOnes = pool.filter(it => !beforeUIDs.has(it.uid));
            return newOnes[newOnes.length - 1] ?? null;
        } catch (err) {
            handleError('Add failed', err);
            return null;
        }
    }, [applySnapshot, handleError, requireSession, snapshot]);

    const removeItem = useCallback(async (uid: string) => {
        const id = requireSession('Remove failed');
        if (!id) return;
        try {
            const next = await RemoveInventoryWorkspaceItem(id, uid);
            applySnapshot(next);
        } catch (err) {
            handleError('Remove failed', err);
        }
    }, [applySnapshot, handleError, requireSession]);

    const updateWeapon = useCallback(async (uid: string, patch: editor.WeaponPatch) => {
        const id = requireSession('Weapon edit failed');
        if (!id) return null;
        try {
            const next = await UpdateInventoryWorkspaceWeapon(id, uid, patch);
            applySnapshot(next);
            return findByUID(next.inventoryItems, uid) ?? findByUID(next.storageItems, uid);
        } catch (err) {
            handleError('Weapon edit failed', err);
            return null;
        }
    }, [applySnapshot, handleError, requireSession]);

    const save = useCallback(async () => {
        const id = requireSession('Save failed');
        if (!id) return null;
        setSaving(true);
        setLastError(null);
        try {
            const next = await SaveInventoryWorkspaceChanges(id);
            applySnapshot(next);
            return next;
        } catch (err) {
            handleError('Save failed', err);
            return null;
        } finally {
            setSaving(false);
        }
    }, [applySnapshot, handleError, requireSession]);

    const discard = useCallback(async () => {
        const id = sessionIDRef.current;
        const charIdx = characterIndexRef.current;
        if (!id || charIdx == null) return;
        setLoading(true);
        try {
            await DiscardInventoryEditSession(id);
            applySnapshot(null);
            await start(charIdx);
        } catch (err) {
            handleError('Discard failed', err);
        } finally {
            setLoading(false);
        }
    }, [applySnapshot, handleError, start]);

    // On unmount, attempt to discard any in-flight session to avoid leaks.
    useEffect(() => {
        return () => {
            const id = sessionIDRef.current;
            if (id) {
                DiscardInventoryEditSession(id).catch(() => { /* best-effort cleanup */ });
            }
        };
    }, []);

    const clearError = useCallback(() => setLastError(null), []);

    const replaceSnapshot = useCallback((snap: editor.InventoryWorkspaceSnapshot) => {
        applySnapshot(snap);
    }, [applySnapshot]);

    return useMemo<UseInventoryWorkspaceResult>(() => ({
        sessionID: snapshot?.sessionID ?? '',
        characterIndex: snapshot?.characterIndex ?? null,
        inventoryItems: snapshot?.inventoryItems ?? [],
        storageItems: snapshot?.storageItems ?? [],
        validation: snapshot?.validation ?? null,
        dirty: snapshot?.dirty ?? false,
        loading,
        saving,
        lastError,
        clearError,
        start,
        refresh,
        moveItem,
        transferItem,
        addItem,
        removeItem,
        updateWeapon,
        save,
        discard,
        replaceSnapshot,
    }), [snapshot, loading, saving, lastError, clearError, start, refresh, moveItem, transferItem, addItem, removeItem, updateWeapon, save, discard, replaceSnapshot]);
}

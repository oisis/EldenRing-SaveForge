import { useState } from 'react';
import toast from '../../lib/toast';
import { main, templates } from '../../../wailsjs/go/models';
import { TemplateLibraryModal } from './TemplateLibraryModal';
import { ImportTemplatePreviewModal } from './ImportTemplatePreviewModal';

// TemplatesShellModal is the global, sidebar-mounted Templates surface.
// Phase 1 scope: library-only. Apply / Import-from-file / Create-from-
// current-workspace stay on the existing SortOrderTab dropdown because
// they require an active InventoryEditSession, which the global shell
// does not own. See spec/56 §6 and §17 Phase 1.
//
// The shell delegates to TemplateLibraryModal in allowApply={false}
// mode and reuses the existing ImportTemplatePreviewModal as a
// read-only viewer when the user clicks Preview on a library entry.

interface Props {
    onClose: () => void;
}

export function TemplatesShellModal({ onClose }: Props) {
    const [previewReport, setPreviewReport] = useState<templates.ImportPreviewReport | null>(null);

    return (
        <>
            <TemplateLibraryModal
                sessionID=""
                allowApply={false}
                title="Templates"
                onClose={onClose}
                onApplied={() => { /* allowApply=false hides the Apply button; this never fires */ }}
                onError={(err) => toast.error(`Templates: ${String(err)}`)}
                onPreviewed={(preview: main.LoadedTemplatePreview) => {
                    setPreviewReport(preview.report);
                }}
                onExportedToFile={(result: main.BuildTemplateExportResult, entry: templates.LibraryTemplateEntry) => {
                    if (result.path) {
                        toast.success(`Template "${entry.name || entry.id}" exported to ${result.path}`);
                    }
                }}
                onDeleted={(id) => toast.success(`Template ${id} deleted from library.`)}
                onRefreshed={(list) => toast.success(`Template library refreshed (${list.length} entries).`)}
            />
            {previewReport && (
                <ImportTemplatePreviewModal
                    report={previewReport}
                    onClose={() => setPreviewReport(null)}
                />
            )}
        </>
    );
}

import {useState} from 'react';
import toast from '../lib/toast';
import {SelectAndOpenSourceSave, GetSourceActiveSlots, ImportCharacter} from '../../wailsjs/go/main/App';
import {RiskActionButton} from './RiskActionButton';

interface Props {
    destSlot: number;
    onComplete: () => void;
}

export function CharacterImporter({destSlot, onComplete}: Props) {
    const [sourceLoaded, setSourceLoaded] = useState(false);
    const [sourceSlots, setSourceSlots] = useState<boolean[]>(new Array(10).fill(false));
    const [selectedSourceSlot, setSelectedSourceSlot] = useState<number | null>(null);
    const [loading, setLoading] = useState(false);

    const handleOpenSource = async () => {
        try {
            const res = await SelectAndOpenSourceSave();
            if (res) {
                const slots = await GetSourceActiveSlots();
                setSourceSlots(slots || new Array(10).fill(false));
                setSourceLoaded(true);
            }
        } catch (e) {
            toast.error("Error: " + e);
        }
    };

    const handleImport = async () => {
        if (selectedSourceSlot === null) return;
        setLoading(true);
        try {
            await ImportCharacter(selectedSourceSlot, destSlot);
            toast.success("Character imported successfully!");
            onComplete();
        } catch (e) {
            toast.error("Import failed: " + e);
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-700">
            <div className="px-4 py-3 rounded-md border border-yellow-500/40 bg-yellow-500/10 text-yellow-400 text-xs font-semibold text-center max-w-2xl mx-auto">
                This feature is temporarily disabled and will be available in a future update.
            </div>
            <div className="card p-10 text-center max-w-2xl mx-auto relative overflow-hidden group opacity-50 pointer-events-none">
                <div className="absolute top-0 right-0 w-32 h-32 bg-primary/5 rounded-full -mr-16 -mt-16 group-hover:bg-primary/10 transition-all duration-1000" />
                
                <div className="relative space-y-6">
                    <div className="w-14 h-14 bg-muted/50 border border-border rounded-2xl flex items-center justify-center mx-auto shadow-inner">
                        <svg className="w-7 h-7 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
                        </svg>
                    </div>
                    
                    <div className="space-y-2">
                        <h3 className="text-xl font-black tracking-tight">Character Importer</h3>
                        <p className="text-sm text-muted-foreground leading-relaxed max-w-sm mx-auto font-medium">
                            Transfer a character profile from an external save file into your current session.
                        </p>
                    </div>
                    
                    {!sourceLoaded ? (
                        <div className="pt-4">
                            <button 
                                onClick={handleOpenSource}
                                className="bg-foreground text-background hover:scale-[1.02] active:scale-[0.98] transition-all font-black px-10 py-3 rounded-md text-[11px] shadow-xl uppercase tracking-[0.2em]"
                            >
                                Select Source File
                            </button>
                        </div>
                    ) : (
                        <div className="space-y-8 pt-4 animate-in slide-in-from-bottom-2 duration-300">
                            <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
                                {sourceSlots.map((isActive, i) => (
                                    <button
                                        key={i}
                                        disabled={!isActive}
                                        onClick={() => setSelectedSourceSlot(i)}
                                        className={`
                                            p-4 rounded-md border transition-all flex flex-col items-center space-y-2 relative
                                            ${!isActive ? 'opacity-30 grayscale cursor-not-allowed border-border bg-muted/50' : 
                                              selectedSourceSlot === i ? 'border-primary bg-primary/5 ring-1 ring-primary' : 
                                              'border-border bg-background hover:border-primary/30'}
                                        `}
                                    >
                                        <span className={`text-[9px] font-black uppercase tracking-widest ${selectedSourceSlot === i ? 'text-primary' : 'text-muted-foreground'}`}>Slot {i + 1}</span>
                                        <div className="w-10 h-10 rounded-full bg-muted/30 border border-border/50 flex items-center justify-center overflow-hidden mb-1 group-hover:border-primary/50 transition-all">
                                            <img 
                                                src="items/armor/knight_helm.png" 
                                                alt="" 
                                                className={`w-7 h-7 object-contain opacity-60 transition-all ${selectedSourceSlot === i ? 'opacity-100 scale-110' : ''}`}
                                            />
                                        </div>
                                        <div className={`w-2 h-2 rounded-full ${isActive ? 'bg-green-500' : 'bg-zinc-300 dark:bg-zinc-800'}`} />
                                        {selectedSourceSlot === i && (
                                            <div className="absolute top-1 right-1">
                                                <svg className="w-3 h-3 text-primary" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd"></path></svg>
                                            </div>
                                        )}
                                    </button>
                                ))}
                            </div>

                            <div className="flex items-center justify-center space-x-6 pt-2">
                                <button 
                                    onClick={() => setSourceLoaded(false)}
                                    className="text-muted-foreground hover:text-foreground text-[10px] font-black uppercase tracking-[0.2em] transition-colors px-4 py-2"
                                >
                                    Cancel
                                </button>
                                <RiskActionButton
                                    riskKey="character_import"
                                    disabled={selectedSourceSlot === null || loading}
                                    onConfirm={handleImport}
                                    className={`
                                        bg-foreground text-background hover:scale-[1.02] active:scale-[0.98] transition-all font-black px-8 py-3 rounded-md text-[11px] shadow-xl uppercase tracking-[0.2em]
                                        ${(selectedSourceSlot === null || loading) ? 'opacity-50 cursor-not-allowed' : ''}
                                    `}
                                >
                                    {loading ? 'Importing...' : `Confirm Import`}
                                </RiskActionButton>
                            </div>
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}

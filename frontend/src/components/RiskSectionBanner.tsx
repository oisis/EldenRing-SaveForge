import {RISK_INFO, RiskKey, RiskLevel} from '../data/riskInfo';
import {RiskInfoIcon} from './RiskInfoIcon';

interface Props {
    riskKey: RiskKey;
    className?: string;
}

const TONE: Record<RiskLevel, string> = {
    low:    'bg-yellow-500/10 border-yellow-500/40',
    medium: 'bg-orange-500/10 border-orange-500/40',
    high:   'bg-red-500/10    border-red-500/40',
};

const TITLE_TONE: Record<RiskLevel, string> = {
    low:    'text-warning-foreground',
    medium: 'text-orange-700',
    high:   'text-destructive',
};

const ICON_TONE: Record<RiskLevel, string> = {
    low:    'text-yellow-600',
    medium: 'text-orange-600',
    high:   'text-red-500',
};

function firstSentence(text: string): string {
    const idx = text.indexOf('. ');
    return idx === -1 ? text : text.slice(0, idx + 1);
}

export function RiskSectionBanner({riskKey, className = ''}: Props) {
    const entry = RISK_INFO[riskKey];
    if (!entry) return null;
    return (
        <div className={`px-3 py-2 rounded border-l-2 flex items-start gap-3 ${TONE[entry.level]} ${className}`}>
            <span className={`text-base leading-none ${ICON_TONE[entry.level]}`}>⚠</span>
            <p className="text-[10px] leading-relaxed flex-1">
                <strong className={`font-black uppercase tracking-widest ${TITLE_TONE[entry.level]}`}>{entry.title}.</strong>{' '}
                <span className="text-foreground/80">{firstSentence(entry.whyBan)}</span>
            </p>
            <RiskInfoIcon riskKey={riskKey} />
        </div>
    );
}

// Threshold colour for the global inventory capacity strip.
// < 80%: default foreground, 80%–95%: orange, strictly > 95%: red.
export function capacityColor(used: number, max: number): string {
    const pct = max > 0 ? (used / max) * 100 : 0;
    if (pct > 95) return 'text-red-400';
    if (pct >= 80) return 'text-orange-400';
    return 'text-foreground';
}

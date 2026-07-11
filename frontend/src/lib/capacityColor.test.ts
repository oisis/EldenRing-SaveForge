import { describe, expect, it } from 'vitest';
import { capacityColor } from './capacityColor';

describe('capacityColor thresholds', () => {
    it('is default foreground below 80%', () => {
        expect(capacityColor(0, 100)).toBe('text-foreground');
        expect(capacityColor(79, 100)).toBe('text-foreground');
    });
    it('is orange at the 80% and 95% boundaries', () => {
        expect(capacityColor(80, 100)).toBe('text-orange-400'); // exactly 80% → orange
        expect(capacityColor(95, 100)).toBe('text-orange-400'); // exactly 95% → orange
    });
    it('is red only strictly above 95%', () => {
        expect(capacityColor(96, 100)).toBe('text-red-400');
        expect(capacityColor(100, 100)).toBe('text-red-400');
    });
    it('treats zero max as empty (foreground)', () => {
        expect(capacityColor(0, 0)).toBe('text-foreground');
    });
});

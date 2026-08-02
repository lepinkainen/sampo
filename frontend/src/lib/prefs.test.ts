import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { readPref, writePref } from './prefs';

const VIEW = ['grid', 'list'] as const;

function memoryStorage() {
	const map = new Map<string, string>();
	return {
		getItem: (k: string) => (map.has(k) ? (map.get(k) as string) : null),
		setItem: (k: string, v: string) => {
			map.set(k, v);
		},
		removeItem: (k: string) => {
			map.delete(k);
		},
		clear: () => map.clear(),
		key: () => null,
		length: 0,
	};
}

beforeEach(() => {
	vi.stubGlobal('localStorage', memoryStorage());
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('readPref', () => {
	it('returns fallback when nothing stored', () => {
		expect(readPref('sampo-view-mode', VIEW, 'grid')).toBe('grid');
	});

	it('returns the stored value when it is an allowed option', () => {
		localStorage.setItem('sampo-view-mode', 'list');
		expect(readPref('sampo-view-mode', VIEW, 'grid')).toBe('list');
	});

	it('falls back when the stored value is not allowed', () => {
		localStorage.setItem('sampo-view-mode', 'bogus');
		expect(readPref('sampo-view-mode', VIEW, 'grid')).toBe('grid');
	});

	it('falls back when localStorage access throws (private mode)', () => {
		vi.stubGlobal('localStorage', {
			getItem: () => {
				throw new Error('storage disabled');
			},
		});
		expect(readPref('sampo-view-mode', VIEW, 'grid')).toBe('grid');
	});
});

describe('writePref', () => {
	it('persists a value readable back through localStorage', () => {
		writePref('sampo-thumb-size', 'large');
		expect(localStorage.getItem('sampo-thumb-size')).toBe('large');
	});

	it('round-trips with readPref', () => {
		writePref('sampo-view-mode', 'list');
		expect(readPref('sampo-view-mode', VIEW, 'grid')).toBe('list');
	});

	it('swallows errors when localStorage access throws', () => {
		vi.stubGlobal('localStorage', {
			setItem: () => {
				throw new Error('storage disabled');
			},
		});
		expect(() => writePref('sampo-thumb-size', 'large')).not.toThrow();
	});
});

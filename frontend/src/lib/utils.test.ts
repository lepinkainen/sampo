import { describe, expect, it } from 'vitest';
import { formatDuration, formatResolution } from './utils';

describe('formatResolution', () => {
	describe('images (non-video)', () => {
		it('renders raw W x H', () => {
			expect(formatResolution(1000, 2000, 'image')).toBe('1000 x 2000');
		});

		it('renders raw for portrait images too', () => {
			expect(formatResolution(1080, 1920, 'image')).toBe('1080 x 1920');
		});

		it('returns empty when dimensions are missing or zero', () => {
			expect(formatResolution(undefined, undefined, 'image')).toBe('');
			expect(formatResolution(0, 0, 'image')).toBe('');
		});

		it('treats unspecified mediaType as non-video (raw)', () => {
			expect(formatResolution(1920, 1080)).toBe('1920 x 1080');
		});
	});

	describe('video landscape', () => {
		const cases: Array<[number, number, string]> = [
			[640, 480, '480p'],
			[1280, 720, '720p'],
			[1920, 1080, '1080p'],
			[2560, 1440, '1440p'],
			[3840, 2160, '4K'],
			[7680, 4320, '8K'],
		];
		for (const [w, h, label] of cases) {
			it(`${w}x${h} -> ${label}`, () => {
				expect(formatResolution(w, h, 'video')).toBe(label);
			});
		}
	});

	describe('video portrait (vertical)', () => {
		const cases: Array<[number, number, string]> = [
			[720, 1280, '720p vertical'],
			[1080, 1920, '1080p vertical'],
			[2160, 3840, '4K vertical'],
		];
		for (const [w, h, label] of cases) {
			it(`${w}x${h} -> ${label}`, () => {
				expect(formatResolution(w, h, 'video')).toBe(label);
			});
		}
	});

	describe('video non-standard aspect (raw fallback)', () => {
		const cases: Array<[number, number, string]> = [
			[2560, 1080, '2560 x 1080'], // ultrawide 21.6:9-ish, 1080 tall but not a standard aspect
			[1920, 800, '1920 x 800'], // cinematic 2.4:1
			[1440, 900, '1440 x 900'], // 16:10 laptop, not bucketed by height
		];
		for (const [w, h, label] of cases) {
			it(`${w}x${h} -> ${label}`, () => {
				expect(formatResolution(w, h, 'video')).toBe(label);
			});
		}
	});

	it('accepts near-bucket heights within tolerance (encoder rounding)', () => {
		// 1920x1088: height 1088 is within tolerance of 1080 and the ratio
		// stays ~16:9, so it still labels as 1080p.
		expect(formatResolution(1920, 1088, 'video')).toBe('1080p');
	});

	it('accepts NTSC SD 720x480 (3:2 is standard)', () => {
		expect(formatResolution(720, 480, 'video')).toBe('480p');
	});
});

describe('formatDuration', () => {
	it('formats seconds', () => {
		expect(formatDuration(9)).toBe('9s');
	});

	it('formats minutes and seconds', () => {
		expect(formatDuration(23 * 60 + 57)).toBe('23m 57s');
	});

	it('formats hours and minutes', () => {
		expect(formatDuration(3600 + 5 * 60)).toBe('1h 5m');
	});

	it('returns empty for zero/missing', () => {
		expect(formatDuration(0)).toBe('');
		expect(formatDuration(undefined)).toBe('');
		expect(formatDuration(null)).toBe('');
	});
});

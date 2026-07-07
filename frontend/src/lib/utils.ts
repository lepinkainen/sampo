import type { FileEntry } from '$lib/types';

export function formatSize(bytes: number): string {
	if (bytes < 1024) return `${bytes} B`;
	if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
	if (bytes < 1024 * 1024 * 1024)
		return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
	return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

export function sortEntries(entries: FileEntry[]): FileEntry[] {
	return entries.sort((a, b) => {
		if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
		return a.name.localeCompare(b.name);
	});
}

export function formatDate(dateStr: string): string {
	try {
		return new Date(dateStr).toISOString().replace('T', ' ').slice(0, 19);
	} catch {
		return dateStr;
	}
}

// Maps the "short edge / reference dimension" to a display label for videos.
// The reference dimension is height for landscape and width for portrait.
const VIDEO_BUCKETS: Record<number, string> = {
	240: '240p',
	360: '360p',
	480: '480p',
	576: '576p',
	720: '720p',
	1080: '1080p',
	1440: '1440p',
	2160: '4K',
	4320: '8K',
};

// Aspect ratios (as numeric w/h) accepted as "standard" for bucket labeling,
// checked by proximity so encoder rounding (e.g. 1920x1088) still classifies.
// Anything else (ultrawide 2.37:1, cinematic 2.4:1, odd crops) falls back to
// raw WxH.
const STANDARD_RATIOS = [16 / 9, 4 / 3, 3 / 2, 9 / 16, 1];

function isStandardAspect(w: number, h: number): boolean {
	const r = w / h;
	return STANDARD_RATIOS.some((std) => Math.abs(r - std) / std < 0.03);
}

/**
 * Format pixel dimensions for display.
 * - Non-video (images): always raw, e.g. "1000 x 2000".
 * - Video: a standard label ("720p", "1080p vertical", "4K") when the
 *   reference dimension matches a known bucket AND the aspect ratio is
 *   standard; otherwise raw "W x H" (e.g. ultrawide / non-standard crops).
 */
export function formatResolution(
	w?: number | null,
	h?: number | null,
	mediaType?: string,
): string {
	if (!w || !h || w <= 0 || h <= 0) return '';
	if (mediaType !== 'video') return `${w} x ${h}`;

	const portrait = h > w;
	const refDim = portrait ? w : h;
	const tolerance = Math.max(30, refDim * 0.02);

	let label = '';
	for (const key of Object.keys(VIDEO_BUCKETS)) {
		const std = Number(key);
		if (Math.abs(std - refDim) <= tolerance) {
			label = VIDEO_BUCKETS[std];
			break;
		}
	}

	if (label && isStandardAspect(w, h)) {
		return portrait ? `${label} vertical` : label;
	}
	return `${w} x ${h}`;
}

/** Format a duration in seconds as e.g. "1h 5m", "23m 57s", "9s". */
export function formatDuration(seconds?: number | null): string {
	if (!seconds || seconds <= 0) return '';
	const total = Math.round(seconds);
	const h = Math.floor(total / 3600);
	const m = Math.floor((total % 3600) / 60);
	const s = total % 60;
	if (h > 0) return `${h}h ${m}m`;
	if (m > 0) return `${m}m ${s}s`;
	return `${s}s`;
}

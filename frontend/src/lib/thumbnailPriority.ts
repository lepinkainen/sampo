import type { FileEntry } from './types';

/**
 * Per-media-type rank. Lower rank runs first. Encoded as a multiple of
 * `rankScale` so it dominates the byte-size tiebreaker: images (fast CPU
 * decode) before pdfs (heavier rasterization) before videos (ffmpeg seek +
 * frame decode, the slow path that blocks the browser's HTTP pool).
 *
 * `rankScale` is 1e15, well above any realistic single-file size (1 PB), so
 * a smaller file in a costlier rank can never preempt a larger file in a
 * cheaper rank. JS doubles stay exact up to 2^53 ≈ 9e15, leaving headroom.
 */
const rankScale = 1e15;
const rankByType: Record<FileEntry['mediaType'], number> = {
	image: 0,
	pdf: 1,
	video: 2,
	archive: 2,
	other: 2,
};

/**
 * Compute the priority used to order thumbnail fetches. Lower = fetched first.
 *
 * Primary key is media-type rank (images before pdfs before videos). Within
 * a rank the raw byte size is the tiebreaker, so a 200KB image preempts a
 * 5MB image and a 2MB video preempts a 500MB video. `size` comes straight
 * from os.Stat on the backend and is already present on the entry when
 * thumbnails are requested, so this never needs an extra round trip.
 */
export function thumbnailPriority(
	entry: Pick<FileEntry, 'mediaType' | 'size'>,
): number {
	return rankByType[entry.mediaType] * rankScale + entry.size;
}

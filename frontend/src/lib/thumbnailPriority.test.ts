import { describe, expect, it } from 'vitest';
import { thumbnailPriority } from './thumbnailPriority';
import type { FileEntry } from './types';

function entry(
	mediaType: FileEntry['mediaType'],
	size: number,
): Pick<FileEntry, 'mediaType' | 'size'> {
	return { mediaType, size };
}

describe('thumbnailPriority', () => {
	it('orders images before pdfs before videos', () => {
		const image = thumbnailPriority(entry('image', 1_000_000));
		const pdf = thumbnailPriority(entry('pdf', 1_000_000));
		const video = thumbnailPriority(entry('video', 1_000_000));
		expect(image).toBeLessThan(pdf);
		expect(pdf).toBeLessThan(video);
	});

	it('uses size as a tiebreaker within the same media type', () => {
		const small = thumbnailPriority(entry('image', 200_000));
		const large = thumbnailPriority(entry('image', 5_000_000));
		expect(small).toBeLessThan(large);
	});

	it('lets a small image beat a small video despite the type offset', () => {
		// The point of the type offset: even a tiny image is cheaper than any
		// video, and a tiny video is still cheaper than a huge video.
		const tinyImage = thumbnailPriority(entry('image', 1024));
		const tinyVideo = thumbnailPriority(entry('video', 1024));
		expect(tinyImage).toBeLessThan(tinyVideo);
	});

	it('treats archive/other with the same cost as video', () => {
		expect(thumbnailPriority(entry('archive', 0))).toBe(
			thumbnailPriority(entry('video', 0)),
		);
		expect(thumbnailPriority(entry('other', 0))).toBe(
			thumbnailPriority(entry('video', 0)),
		);
	});

	it('produces a strict ordering for a mixed folder matching the intended run order', () => {
		const priorities = [
			thumbnailPriority(entry('video', 500_000_000)),
			thumbnailPriority(entry('image', 5_000_000)),
			thumbnailPriority(entry('video', 50_000_000)),
			thumbnailPriority(entry('pdf', 1_000_000)),
			thumbnailPriority(entry('image', 200_000)),
		];
		const sorted = [...priorities].sort((a, b) => a - b);
		// 200KB image < 5MB image < 1MB pdf < 50MB video < 500MB video
		expect(sorted).toEqual([
			priorities[4],
			priorities[1],
			priorities[3],
			priorities[2],
			priorities[0],
		]);
	});
});

import { render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import type { FileEntry } from '$lib/types';
import ListView from './ListView.svelte';

function entry(over: Partial<FileEntry> = {}): FileEntry {
	return {
		name: 'photo.jpg',
		path: 'photo.jpg',
		isDir: false,
		isZip: false,
		size: 1234,
		modTime: '2026-07-09T12:11:25Z',
		mediaType: 'image',
		hasThumb: true,
		...over,
	};
}

function makeProps(entries: FileEntry[]) {
	return {
		rootId: 'root-0',
		entries,
		isSelected: () => false,
		isCut: () => false,
		onclick: vi.fn(),
		ondblclick: vi.fn(),
		oncontextmenu: vi.fn(),
		ondragstart: vi.fn(),
	};
}

describe('ListView', () => {
	it('renders a thumbnail image for an image entry that has a thumb', () => {
		render(ListView, makeProps([entry({ name: 'photo.jpg' })]));
		const img = screen.getByRole('img', { name: 'photo.jpg' });
		expect(img).toBeInTheDocument();
		expect(img.getAttribute('src')).toContain('/api/thumb/root-0/');
		expect(img.getAttribute('loading')).toBe('lazy');
	});

	it('renders a thumbnail for videos with a thumb', () => {
		render(
			ListView,
			makeProps([entry({ name: 'clip.mp4', mediaType: 'video' })]),
		);
		expect(screen.getByRole('img', { name: 'clip.mp4' })).toBeInTheDocument();
	});

	it('falls back to an icon (no thumbnail img) for directories', () => {
		render(
			ListView,
			makeProps([
				entry({
					name: 'folder',
					isDir: true,
					mediaType: 'other',
					hasThumb: false,
				}),
			]),
		);
		expect(screen.queryByRole('img')).not.toBeInTheDocument();
	});

	it('falls back to an icon when an image has no thumbnail yet', () => {
		render(ListView, makeProps([entry({ name: 'raw.png', hasThumb: false })]));
		expect(screen.queryByRole('img')).not.toBeInTheDocument();
	});

	it('keeps the modified timestamp on a single line (compact rows)', () => {
		render(ListView, makeProps([entry()]));
		const modCell = screen.getByText(/2026-07-09/);
		expect(modCell).toHaveClass('whitespace-nowrap');
	});
});

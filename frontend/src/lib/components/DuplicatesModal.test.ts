import { render, screen, waitFor } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import type { DuplicatesResponse } from '$lib/types';
import DuplicatesModal from './DuplicatesModal.svelte';

type FindOpts = { similar?: boolean; threshold?: number };
const findDuplicates =
	vi.fn<
		(
			rootId: string,
			path: string,
			opts?: FindOpts,
		) => Promise<DuplicatesResponse>
	>();
vi.mock('$lib/api', () => ({
	findDuplicates: (rootId: string, path: string, opts?: FindOpts) =>
		findDuplicates(rootId, path, opts),
	thumbnailUrl: (rootId: string, path: string) =>
		`/api/thumb/${rootId}/${path}`,
	deleteFiles: vi.fn(),
}));

describe('DuplicatesModal', () => {
	it('shows loading then renders groups', async () => {
		findDuplicates.mockResolvedValue({
			groups: [
				{
					hash: 'abcdef0123456789aaaa',
					hashType: 'sha256',
					size: 2048,
					files: [
						{ rootId: 'root-0', path: 'a/one.jpg' },
						{ rootId: 'root-0', path: 'b/two.jpg' },
					],
				},
			],
		});

		render(DuplicatesModal, {
			rootId: 'root-0',
			path: 'pics',
			onClose: vi.fn(),
		});

		expect(screen.getByText('Searching for duplicates...')).toBeInTheDocument();
		await waitFor(() => {
			expect(screen.getByText('one.jpg')).toBeInTheDocument();
		});
		expect(screen.getByText('two.jpg')).toBeInTheDocument();
		expect(findDuplicates).toHaveBeenCalledWith('root-0', 'pics', {
			similar: true,
			threshold: 88,
		});
	});

	it('pre-selects all but the first copy in each group', async () => {
		findDuplicates.mockResolvedValue({
			groups: [
				{
					hash: 'abcdef0123456789aaaa',
					hashType: 'sha256',
					size: 2048,
					files: [
						{ rootId: 'root-0', path: 'a/one.jpg' },
						{ rootId: 'root-0', path: 'b/two.jpg' },
						{ rootId: 'root-0', path: 'c/three.jpg' },
					],
				},
			],
		});

		const { container } = render(DuplicatesModal, {
			rootId: 'root-0',
			path: 'pics',
			onClose: vi.fn(),
		});
		await waitFor(() => {
			expect(screen.getByText('one.jpg')).toBeInTheDocument();
		});

		const checkboxes = [
			...container.querySelectorAll<HTMLInputElement>('input[type=checkbox]'),
		];
		expect(checkboxes.map((c) => c.checked)).toEqual([false, true, true]);
		// Footer summarizes the pre-selection: 2 files, 2 × 2048 bytes.
		expect(screen.getByText('Delete 2 files…')).toBeInTheDocument();
		// Shown twice: once in the group header, once in the footer summary.
		expect(screen.getAllByText(/reclaim 4.0 KB/)).toHaveLength(2);
	});

	it('renders similar groups with badge, similarity, and keeper', async () => {
		findDuplicates.mockResolvedValue({
			groups: [
				{
					hash: 'd8e4a2f1c0b39876',
					hashType: 'phash',
					size: 0,
					maxDistance: 3,
					keeper: 0,
					files: [
						{
							rootId: 'root-0',
							path: 'pics/orig.png',
							size: 6100000,
							width: 4032,
							height: 3024,
							similarity: 100,
						},
						{
							rootId: 'root-0',
							path: 'pics/web.jpg',
							size: 1400000,
							width: 1920,
							height: 1440,
							similarity: 95,
						},
					],
				},
			],
		});

		const { container } = render(DuplicatesModal, {
			rootId: 'root-0',
			path: 'pics',
			onClose: vi.fn(),
		});
		await waitFor(() => {
			expect(screen.getByText('orig.png')).toBeInTheDocument();
		});

		expect(screen.getByText('SIMILAR')).toBeInTheDocument();
		expect(screen.getByText(/±3 bits/)).toBeInTheDocument();
		expect(screen.getByText('95% match')).toBeInTheDocument();
		expect(screen.getByText(/4032×3024/)).toHaveTextContent('← best');
		// Auto-select keeps the high-res keeper, marks the smaller copy.
		const checkboxes = [
			...container.querySelectorAll<HTMLInputElement>('input[type=checkbox]'),
		];
		expect(checkboxes.map((c) => c.checked)).toEqual([false, true]);
		expect(screen.getByText('KEEP')).toBeInTheDocument();
	});

	it('leaves quality ties unselected until KEEP THIS resolves them', async () => {
		findDuplicates.mockResolvedValue({
			groups: [
				{
					hash: '9f22c1aabbccdd00',
					hashType: 'phash',
					size: 0,
					maxDistance: 1,
					files: [
						{
							rootId: 'root-0',
							path: 'scans/poster.png',
							size: 14800000,
							width: 3600,
							height: 2400,
							similarity: 100,
						},
						{
							rootId: 'root-1',
							path: 'misc/poster.webp',
							size: 1200000,
							width: 3600,
							height: 2400,
							similarity: 98,
						},
					],
				},
			],
		});

		const { container } = render(DuplicatesModal, {
			rootId: 'root-0',
			path: 'scans',
			onClose: vi.fn(),
		});
		await waitFor(() => {
			expect(screen.getByText('poster.png')).toBeInTheDocument();
		});

		expect(screen.getByText('QUALITY TIE')).toBeInTheDocument();
		// No auto-selection for ties.
		const checkboxes = [
			...container.querySelectorAll<HTMLInputElement>('input[type=checkbox]'),
		];
		expect(checkboxes.map((c) => c.checked)).toEqual([false, false]);

		// Picking a keeper marks the other file.
		const keepButtons = screen.getAllByText('KEEP THIS');
		expect(keepButtons).toHaveLength(2);
		keepButtons[0].click();
		await waitFor(() => {
			const boxes = [
				...container.querySelectorAll<HTMLInputElement>('input[type=checkbox]'),
			];
			expect(boxes.map((c) => c.checked)).toEqual([false, true]);
		});
		expect(screen.getByText('✓ KEEPING')).toBeInTheDocument();
	});

	it('filters groups by tab', async () => {
		findDuplicates.mockResolvedValue({
			groups: [
				{
					hash: 'exacthash',
					hashType: 'sha256',
					size: 2048,
					files: [
						{ rootId: 'root-0', path: 'a/one.jpg' },
						{ rootId: 'root-0', path: 'b/two.jpg' },
					],
				},
				{
					hash: 'd8e4a2f1c0b39876',
					hashType: 'phash',
					size: 0,
					keeper: 0,
					files: [
						{
							rootId: 'root-0',
							path: 'c/big.png',
							size: 100,
							width: 200,
							height: 100,
							similarity: 100,
						},
						{
							rootId: 'root-0',
							path: 'd/small.jpg',
							size: 50,
							width: 100,
							height: 50,
							similarity: 96,
						},
					],
				},
			],
		});

		render(DuplicatesModal, {
			rootId: 'root-0',
			path: 'pics',
			onClose: vi.fn(),
		});
		await waitFor(() => {
			expect(screen.getByText('one.jpg')).toBeInTheDocument();
		});
		expect(screen.getByText('big.png')).toBeInTheDocument();

		// Exact tab hides the phash group.
		screen.getByRole('button', { name: /Exact/ }).click();
		await waitFor(() => {
			expect(screen.queryByText('big.png')).not.toBeInTheDocument();
		});
		expect(screen.getByText('one.jpg')).toBeInTheDocument();

		// Similar tab hides the exact group.
		screen.getByRole('button', { name: /Similar/ }).click();
		await waitFor(() => {
			expect(screen.queryByText('one.jpg')).not.toBeInTheDocument();
		});
		expect(screen.getByText('big.png')).toBeInTheDocument();
	});

	it('renders empty state when no duplicates', async () => {
		findDuplicates.mockResolvedValue({ groups: [] });
		render(DuplicatesModal, { rootId: 'root-0', path: '', onClose: vi.fn() });
		await waitFor(() => {
			expect(screen.getByText('No duplicates found')).toBeInTheDocument();
		});
		// Empty path falls back to root.
		expect(findDuplicates).toHaveBeenCalledWith('root-0', '/', {
			similar: true,
			threshold: 88,
		});
	});

	it('fires onClose from the close button', async () => {
		findDuplicates.mockResolvedValue({ groups: [] });
		const onClose = vi.fn();
		const { container } = render(DuplicatesModal, {
			rootId: 'root-0',
			path: '',
			onClose,
		});
		await waitFor(() =>
			expect(screen.getByText('No duplicates found')).toBeInTheDocument(),
		);
		const closeBtn = container.querySelector<HTMLButtonElement>(
			'button[title="Close"]',
		);
		closeBtn?.click();
		expect(onClose).toHaveBeenCalled();
	});
});

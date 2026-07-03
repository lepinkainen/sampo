import { render, screen, waitFor } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import type { DuplicatesResponse } from '$lib/types';
import DuplicatesModal from './DuplicatesModal.svelte';

const findDuplicates =
	vi.fn<(rootId: string, path: string) => Promise<DuplicatesResponse>>();
vi.mock('$lib/api', () => ({
	findDuplicates: (rootId: string, path: string) =>
		findDuplicates(rootId, path),
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
		expect(findDuplicates).toHaveBeenCalledWith('root-0', 'pics');
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

	it('renders empty state when no duplicates', async () => {
		findDuplicates.mockResolvedValue({ groups: [] });
		render(DuplicatesModal, { rootId: 'root-0', path: '', onClose: vi.fn() });
		await waitFor(() => {
			expect(screen.getByText('No duplicates found')).toBeInTheDocument();
		});
		// Empty path falls back to root.
		expect(findDuplicates).toHaveBeenCalledWith('root-0', '/');
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

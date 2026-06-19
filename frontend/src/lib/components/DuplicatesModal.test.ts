import { render, screen, waitFor } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import type { DuplicatesResponse } from '$lib/types';
import DuplicatesModal from './DuplicatesModal.svelte';

const findDuplicates =
	vi.fn<(rootId: string, path: string) => Promise<DuplicatesResponse>>();
vi.mock('$lib/api', () => ({
	findDuplicates: (rootId: string, path: string) =>
		findDuplicates(rootId, path),
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
			expect(screen.getByText('a/one.jpg')).toBeInTheDocument();
		});
		expect(screen.getByText('b/two.jpg')).toBeInTheDocument();
		expect(findDuplicates).toHaveBeenCalledWith('root-0', 'pics');
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
		const buttons = container.querySelectorAll('button');
		buttons[0].click();
		expect(onClose).toHaveBeenCalled();
	});
});

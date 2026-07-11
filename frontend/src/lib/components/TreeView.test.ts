import { fireEvent, render, screen } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { FileEntry, Root } from '$lib/types';
import TreeNode from './TreeNode.svelte';
import TreeView from './TreeView.svelte';

const roots: Root[] = [{ id: 'root-0', name: 'Sample' }];

// Minimal fake filesystem keyed by "rootId:path".
const fs: Record<string, FileEntry[]> = {
	'root-0:/': [dir('nesterally2015', '/nesterally2015')],
	'root-0:/nesterally2015': [dir('sub', '/nesterally2015/sub')],
	'root-0:/nesterally2015/sub': [dir('leaf', '/nesterally2015/sub/leaf')],
};

function dir(name: string, path: string): FileEntry {
	return {
		name,
		path,
		isDir: true,
		isZip: false,
		size: 0,
		modTime: '',
		mediaType: 'other',
		hasThumb: false,
	};
}

const fetchDirectory = vi.fn((rootId: string, path: string) =>
	Promise.resolve(fs[`${rootId}:${path}`] ?? []),
);

vi.mock('$lib/api', () => ({
	fetchRoots: () => Promise.resolve(roots),
	fetchDirectory: (rootId: string, path: string) =>
		fetchDirectory(rootId, path),
	moveFiles: vi.fn(),
	copyFiles: vi.fn(),
	deleteFiles: vi.fn(),
	renameFile: vi.fn(),
	invalidateDirectoryCache: vi.fn(),
	invalidateParentDirectoryCache: vi.fn(),
}));

describe('TreeView URL-driven expansion', () => {
	beforeEach(() => {
		fetchDirectory.mockClear();
	});

	it('does not expand any root when no path is selected', async () => {
		render(TreeView, { selectedPath: null, onSelect: vi.fn() });
		// Root button renders, but its children are never fetched.
		expect(await screen.findByText('Sample')).toBeInTheDocument();
		expect(screen.queryByText('nesterally2015')).not.toBeInTheDocument();
		expect(fetchDirectory).not.toHaveBeenCalled();
	});

	it('auto-expands the tree down to the URL-selected directory on load', async () => {
		render(TreeView, {
			selectedPath: 'root-0:/nesterally2015/sub',
			onSelect: vi.fn(),
		});

		// Cascade should reveal every ancestor and the selected dir itself.
		expect(await screen.findByText('nesterally2015')).toBeInTheDocument();
		expect(await screen.findByText('sub')).toBeInTheDocument();

		// Only the path to the selection is loaded: root, nesterally2015.
		// The selected node itself is not auto-expanded, so its children
		// ("leaf") stay collapsed and unfetched.
		expect(screen.queryByText('leaf')).not.toBeInTheDocument();
		expect(fetchDirectory).toHaveBeenCalledWith('root-0', '/');
		expect(fetchDirectory).toHaveBeenCalledWith('root-0', '/nesterally2015');
		expect(fetchDirectory).not.toHaveBeenCalledWith(
			'root-0',
			'/nesterally2015/sub',
		);
	});

	it('highlights the URL-selected directory', async () => {
		render(TreeView, {
			selectedPath: 'root-0:/nesterally2015/sub',
			onSelect: vi.fn(),
		});
		const selected = await screen.findByText('sub');
		// isSelected applies the active bg/text classes on the row button.
		expect(selected.closest('button')).toHaveClass('bg-select');
	});
});

describe('TreeNode context menu opt-in', () => {
	beforeEach(() => {
		fetchDirectory.mockClear();
	});

	it('shows a context menu on right-click when reached via TreeView (enabled by default)', async () => {
		render(TreeView, { selectedPath: null, onSelect: vi.fn() });
		const rootButton = await screen.findByText('Sample');
		await fireEvent.click(rootButton);
		const node = await screen.findByText('nesterally2015');

		await fireEvent.contextMenu(node.closest('button')!);

		expect(await screen.findByText('Rename')).toBeInTheDocument();
		expect(screen.getByText('Cut')).toBeInTheDocument();
		expect(screen.getByText('Copy')).toBeInTheDocument();
		expect(screen.getByText('Paste')).toBeInTheDocument();
		expect(screen.getByText('Delete')).toBeInTheDocument();
	});

	it('does NOT show a context menu when enableContextMenu is left unset (OrganizeModal-style usage)', async () => {
		const entry: FileEntry = dir('nesterally2015', '/nesterally2015');
		render(TreeNode, {
			rootId: 'root-0',
			entry,
			depth: 0,
			selectedPath: null,
			onSelect: vi.fn(),
			// enableContextMenu intentionally omitted, matching how
			// OrganizeModal instantiates TreeNode as a read-only picker.
		});

		const node = await screen.findByText('nesterally2015');
		await fireEvent.contextMenu(node.closest('button')!);

		expect(screen.queryByText('Rename')).not.toBeInTheDocument();
		expect(screen.queryByText('Delete')).not.toBeInTheDocument();
	});
});

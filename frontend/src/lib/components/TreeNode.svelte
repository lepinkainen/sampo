<script lang="ts">
import {
	fetchDirectory,
	moveFiles,
	copyFiles,
	deleteFiles,
	renameFile,
	invalidateDirectoryCache,
	invalidateParentDirectoryCache,
} from '$lib/api';
import { showToast, summarizeItemErrors } from '$lib/toast.svelte';
import type { FileEntry } from '$lib/types';
import type { ClipboardStore } from '$lib/clipboard.svelte';
import { sortEntries } from '$lib/utils';
import {
	ChevronDown,
	ChevronRight,
	Scissors,
	Copy,
	ClipboardPaste,
	Trash2,
	Pencil,
} from '@lucide/svelte';
import FileIcon from './FileIcon.svelte';
import Loader from './Loader.svelte';
import ContextMenu from './ContextMenu.svelte';
import ConfirmDialog from './ConfirmDialog.svelte';
import RenameDialog from './RenameDialog.svelte';
import TreeNode from './TreeNode.svelte';

interface Props {
	rootId: string;
	entry: FileEntry;
	depth?: number;
	selectedPath: string | null;
	onSelect: (rootId: string, path: string, isDir: boolean) => void;
	onRefresh?: () => void;
	// Opt-in: only TreeView (the main sidebar tree) enables the context menu
	// and its file operations. OrganizeModal reuses TreeNode purely as a
	// read-only directory picker and must stay inert, so this defaults to
	// false and OrganizeModal never passes it.
	enableContextMenu?: boolean;
	clipboard?: ClipboardStore;
	// Called after an operation removes/renames THIS node from its parent's
	// child list (self cut/delete) so the parent can refresh its listing.
	onRefreshParent?: () => void;
	// Bubbles a rename/move/delete of a specific rootId+path up to +page.svelte
	// so it can update the URL/selection if the currently browsed directory
	// was affected. newPath === null means deleted.
	onPathChanged?: (
		rootId: string,
		oldPath: string,
		newPath: string | null,
	) => void;
}

let {
	rootId,
	entry,
	depth = 0,
	selectedPath,
	onSelect,
	onRefresh,
	enableContextMenu = false,
	clipboard,
	onRefreshParent,
	onPathChanged,
}: Props = $props();

let expanded = $state(false);
let children = $state<FileEntry[]>([]);
let loading = $state(false);
let loadingSlow = $state(false);
let loadingTimer: ReturnType<typeof setTimeout> | null = null;
let dragOver = $state(false);

let contextMenu = $state<{ x: number; y: number } | null>(null);
let showDeleteConfirm = $state(false);
let showRenameDialog = $state(false);

const isSelected = $derived(selectedPath === `${rootId}:${entry.path}`);

$effect(() => {
	return () => {
		if (loadingTimer) {
			clearTimeout(loadingTimer);
			loadingTimer = null;
		}
	};
});

// Auto-expand this node when it is an ancestor of the URL-selected path,
// cascading the tree open down to the selected directory. Runs once per
// target so a manual collapse afterwards is not re-fought.
let lastAutoTarget: string | null = null;
$effect(() => {
	if (!entry.isDir || !selectedPath) return;
	if (lastAutoTarget === selectedPath || expanded) return;
	const idx = selectedPath.indexOf(':');
	if (idx < 0) return;
	if (selectedPath.slice(0, idx) !== rootId) return;
	const targetPath = selectedPath.slice(idx + 1);
	if (targetPath === entry.path) return; // selected node itself, nothing to open
	if (!targetPath.startsWith(`${entry.path}/`)) return; // not an ancestor
	lastAutoTarget = selectedPath;
	autoExpand();
});

// Shared fetch-and-set-children logic, used by toggle()/autoExpand() (initial
// load) and by the op handlers below (reload after a mutation touches this
// node's own contents).
async function refreshChildren() {
	loading = true;
	loadingSlow = false;
	if (loadingTimer) clearTimeout(loadingTimer);
	loadingTimer = setTimeout(() => {
		loadingSlow = true;
	}, 3000);
	try {
		children = sortEntries(await fetchDirectory(rootId, entry.path)).filter(
			(e) => e.isDir,
		);
	} catch (e) {
		console.error('Failed to load directory', e);
	}
	if (loadingTimer) {
		clearTimeout(loadingTimer);
		loadingTimer = null;
	}
	loading = false;
	loadingSlow = false;
}

async function autoExpand() {
	if (expanded || loading) return;
	await refreshChildren();
	expanded = true;
}

async function toggle() {
	if (!entry.isDir) {
		onSelect(rootId, entry.path, false);
		return;
	}

	if (!expanded) {
		if (loading) return;
		await refreshChildren();
	}

	expanded = !expanded;
	onSelect(rootId, entry.path, true);
}

function handleDragOver(e: DragEvent) {
	if (!entry.isDir) return;
	e.preventDefault();
	e.stopPropagation();
	dragOver = true;
	if (e.dataTransfer) {
		e.dataTransfer.dropEffect = e.altKey ? 'copy' : 'move';
	}
}

function handleDragLeave() {
	dragOver = false;
}

async function handleDrop(e: DragEvent) {
	e.preventDefault();
	e.stopPropagation();
	dragOver = false;

	const data = e.dataTransfer?.getData('application/json');
	if (!data) return;

	let payload: { rootId: string; paths: string[]; mode: 'move' | 'copy' };
	try {
		payload = JSON.parse(data) as typeof payload;
	} catch {
		// ignore invalid drag data
		return;
	}

	try {
		const op = payload.mode === 'copy' ? copyFiles : moveFiles;
		const results = await op({
			items: payload.paths.map((p: string) => ({
				srcRoot: payload.rootId,
				srcPath: p,
			})),
			dstRoot: rootId,
			dstPath: entry.path,
		});
		const summary = summarizeItemErrors(results);
		if (summary) {
			showToast(`Failed to ${payload.mode}: ${summary}`, 'error');
		}
		onRefresh?.();
	} catch (err) {
		showToast(err instanceof Error ? err.message : 'Move failed', 'error');
	}
}

function handleContextMenu(e: MouseEvent) {
	if (!enableContextMenu || !entry.isDir) return;
	e.preventDefault();
	e.stopPropagation();
	contextMenu = { x: e.clientX, y: e.clientY };
}

async function handleDelete() {
	showDeleteConfirm = false;
	try {
		await deleteFiles(rootId, [entry.path], true);
		showToast('Deleted 1 item(s)', 'success');
		if (clipboard?.mode === 'cut' && clipboard.isCut(rootId, entry.path)) {
			clipboard.clear();
		}
		invalidateParentDirectoryCache(rootId, entry.path);
		onRefreshParent?.();
		onPathChanged?.(rootId, entry.path, null);
		onRefresh?.();
	} catch (e) {
		showToast(e instanceof Error ? e.message : 'Delete failed', 'error');
	}
}

function handleCut() {
	if (!clipboard) return;
	clipboard.cut(rootId, [entry.path]);
	showToast('Cut 1 item(s)', 'success');
}

function handleCopy() {
	if (!clipboard) return;
	clipboard.copy(rootId, [entry.path]);
	showToast('Copied 1 item(s)', 'success');
}

async function handlePaste() {
	if (!clipboard?.hasItems) return;
	const op = clipboard.mode === 'cut' ? moveFiles : copyFiles;
	const mode = clipboard.mode;
	try {
		const results = await op({
			items: clipboard.items.map((i) => ({
				srcRoot: i.rootId,
				srcPath: i.path,
			})),
			dstRoot: rootId,
			dstPath: entry.path || '/',
		});
		const summary = summarizeItemErrors(results);
		if (summary) {
			showToast(`Failed to ${mode}: ${summary}`, 'error');
		} else {
			showToast(
				`${mode === 'cut' ? 'Moved' : 'Copied'} ${results.length} item(s)`,
				'success',
			);
		}
		if (mode === 'cut') {
			for (const item of clipboard.items) {
				invalidateParentDirectoryCache(item.rootId, item.path);
			}
			clipboard.clear();
		}
		invalidateDirectoryCache(rootId, entry.path);
		if (expanded) {
			await refreshChildren();
		}
		onRefresh?.();
	} catch (e) {
		showToast(e instanceof Error ? e.message : 'Paste failed', 'error');
	}
}

async function handleRename(newName: string) {
	showRenameDialog = false;
	const oldPath = entry.path;
	try {
		await renameFile(rootId, oldPath, newName);
		showToast(`Renamed to "${newName}"`, 'success');
		invalidateParentDirectoryCache(rootId, oldPath);
		const parts = oldPath.split('/');
		parts.pop();
		const newPath = [...parts, newName].join('/');
		// The node stays in place but its name/path changed; ask the parent to
		// re-fetch its listing (simplest correct approach — this node's own
		// `entry` prop is owned by the parent and can't be mutated in place).
		onRefreshParent?.();
		onPathChanged?.(rootId, oldPath, newPath);
		onRefresh?.();
	} catch (e) {
		showToast(e instanceof Error ? e.message : 'Rename failed', 'error');
	}
}

function getContextMenuItems() {
	const hasClipboard = clipboard?.hasItems ?? false;
	return [
		{
			label: 'Rename',
			icon: Pencil,
			action: () => {
				showRenameDialog = true;
			},
		},
		{
			label: 'Cut',
			icon: Scissors,
			action: handleCut,
		},
		{
			label: 'Copy',
			icon: Copy,
			action: handleCopy,
		},
		{
			label: 'Paste',
			icon: ClipboardPaste,
			action: handlePaste,
			disabled: !hasClipboard,
		},
		{
			label: 'Delete',
			icon: Trash2,
			action: () => {
				showDeleteConfirm = true;
			},
			destructive: true,
		},
	];
}
</script>

<div class="select-none">
	<button
		class="flex min-w-0 w-full items-center gap-1 rounded px-1 py-0.5 text-left text-sm hover:bg-muted
		{isSelected ? 'bg-select text-select-text' : 'text-body'}
		{dragOver ? 'ring-2 ring-accent-hover bg-accent-deep/20' : ''}"
		style="padding-left: {depth * 16 + 4}px"
		onclick={toggle}
		oncontextmenu={handleContextMenu}
		ondragover={handleDragOver}
		ondragleave={handleDragLeave}
		ondrop={handleDrop}
	>
		{#if entry.isDir}
			<span class="w-4 shrink-0 text-faint">
				{#if loading}
					<Loader size={14} />
				{:else if expanded}
					<ChevronDown size={14} />
				{:else}
					<ChevronRight size={14} />
				{/if}
			</span>
		{:else}
			<span class="w-4 shrink-0"></span>
		{/if}

		<span class="w-5 shrink-0 text-dim">
			<FileIcon {entry} open={expanded} size={16} />
		</span>
		<span class="truncate">{entry.name}</span>
		{#if loadingSlow}
			<span class="ml-1 shrink-0 text-xs text-warn">(network drive...)</span>
		{/if}
	</button>

	{#if expanded && children.length > 0}
		{#each children as child (child.path)}
			<TreeNode
				{rootId}
				entry={child}
				depth={depth + 1}
				{selectedPath}
				{onSelect}
				{onRefresh}
				{enableContextMenu}
				{clipboard}
				onRefreshParent={refreshChildren}
				{onPathChanged}
			/>
		{/each}
	{/if}
</div>

{#if enableContextMenu && contextMenu}
	<ContextMenu
		x={contextMenu.x}
		y={contextMenu.y}
		items={getContextMenuItems()}
		onClose={() => (contextMenu = null)}
	/>
{/if}

{#if enableContextMenu && showDeleteConfirm}
	<ConfirmDialog
		title="Delete {entry.name}?"
		items={[{ name: entry.name }]}
		onConfirm={handleDelete}
		onCancel={() => (showDeleteConfirm = false)}
	/>
{/if}

{#if enableContextMenu && showRenameDialog}
	<RenameDialog
		currentName={entry.name}
		onConfirm={handleRename}
		onCancel={() => (showRenameDialog = false)}
	/>
{/if}

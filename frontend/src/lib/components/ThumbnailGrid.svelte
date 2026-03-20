<script lang="ts">
import {
	deleteFiles,
	fetchDirectory,
	moveFiles,
	copyFiles,
	renameFile,
	thumbnailUrl,
	startScan,
	getScanStatus,
	getDetection,
} from '$lib/api';
import type { ScanStatus, DetectionResult } from '$lib/api';
import type { FileEntry } from '$lib/types';
import { formatSize, sortEntries } from '$lib/utils';
import { createSelection } from '$lib/selection.svelte';
import { createClipboard } from '$lib/clipboard.svelte';
import FileIcon from './FileIcon.svelte';
import MediaPreview from './MediaPreview.svelte';
import ThumbnailCard from './ThumbnailCard.svelte';
import ConfirmDialog from './ConfirmDialog.svelte';
import RenameDialog from './RenameDialog.svelte';
import ContextMenu from './ContextMenu.svelte';
import Toast from './Toast.svelte';
import {
	Trash2,
	Scissors,
	Copy,
	ClipboardPaste,
	FolderOpen,
	Pencil,
	UserX,
	ScanSearch,
} from '@lucide/svelte';

interface Props {
	rootId: string;
	path: string;
	previewFile?: string | null;
	onNavigate?: (path: string) => void;
	onPreviewChange?: (path: string | null) => void;
}

let {
	rootId,
	path,
	previewFile = null,
	onNavigate,
	onPreviewChange,
}: Props = $props();

let entries = $state<FileEntry[]>([]);
let loading = $state(false);
let error = $state<string | null>(null);
let thumbSize = $state<'small' | 'medium' | 'large'>('medium');
let savedScrollTop = $state(0);
let scrollContainer: HTMLDivElement | undefined = $state();

const selection = createSelection();
const clipboard = createClipboard();

let showDeleteConfirm = $state(false);
let showRenameDialog = $state(false);
let contextMenu = $state<{ x: number; y: number } | null>(null);
let toastComponent: Toast | undefined = $state();
let filterPeople = $state(false);
let scanStatus = $state<ScanStatus | null>(null);
let scanPollTimer: ReturnType<typeof setInterval> | null = null;
let detectionResult = $state<DetectionResult | null>(null);

let mediaEntries = $derived(
	entries.filter((e) => e.mediaType === 'image' || e.mediaType === 'video'),
);

let previewIndex = $derived.by(() => {
	return previewFile && mediaEntries.length > 0
		? mediaEntries.findIndex((e) => e.path === previewFile)
		: -1;
});

let gridClass = $derived.by(() => {
	switch (thumbSize) {
		case 'small':
			return 'grid-cols-[repeat(auto-fill,minmax(120px,1fr))]';
		case 'medium':
			return 'grid-cols-[repeat(auto-fill,minmax(180px,1fr))]';
		case 'large':
			return 'grid-cols-[repeat(auto-fill,minmax(280px,1fr))]';
	}
});

let selectedEntries = $derived(entries.filter((e) => selection.has(e.path)));

$effect(() => {
	loadDirectory(rootId, path);
});

$effect(() => {
	return () => {
		if (scanPollTimer) {
			clearInterval(scanPollTimer);
			scanPollTimer = null;
		}
	};
});

$effect(() => {
	detectionResult = null;
	if (
		selectedEntries.length === 1 &&
		selectedEntries[0].mediaType === 'image'
	) {
		const sel = selectedEntries[0];
		getDetection(rootId, sel.path)
			.then((r) => (detectionResult = r))
			.catch(() => (detectionResult = null));
	}
});

$effect(() => {
	if (!previewFile && scrollContainer && savedScrollTop > 0) {
		const scrollTarget = savedScrollTop;
		requestAnimationFrame(() => {
			if (scrollContainer) {
				scrollContainer.scrollTop = scrollTarget;
			}
		});
	}
});

async function loadDirectory(rid: string, p: string) {
	selection.clear();
	savedScrollTop = 0;
	loading = true;
	error = null;
	try {
		const result = await fetchDirectory(
			rid,
			p,
			filterPeople ? { filter: 'no-people' } : undefined,
		);
		entries = sortEntries(result);
	} catch (e) {
		error = e instanceof Error ? e.message : 'Failed to load directory';
		entries = [];
	}
	loading = false;
}

function handleClick(entry: FileEntry, e: MouseEvent) {
	if (e.metaKey || e.ctrlKey) {
		selection.toggle(entry);
	} else if (e.shiftKey) {
		selection.selectRange(entries, entry);
	} else {
		selection.selectOne(entry);
	}
}

function handleOpen(entry: FileEntry) {
	if (entry.isDir) {
		onNavigate?.(entry.path);
	} else if (entry.mediaType === 'image' || entry.mediaType === 'video') {
		if (scrollContainer) {
			savedScrollTop = scrollContainer.scrollTop;
		}
		selection.clear();
		onPreviewChange?.(entry.path);
	}
}

function handleContextMenu(e: MouseEvent, entry: FileEntry) {
	e.preventDefault();
	if (!selection.has(entry.path)) {
		selection.selectOne(entry);
	}
	contextMenu = { x: e.clientX, y: e.clientY };
}

function handleKeydown(e: KeyboardEvent) {
	if (showDeleteConfirm || showRenameDialog || previewFile) return;

	const mod = e.metaKey || e.ctrlKey;

	if (e.key === 'Delete' || (e.key === 'Backspace' && mod)) {
		if (selection.size > 0) {
			e.preventDefault();
			showDeleteConfirm = true;
		}
	} else if (mod && e.key === 'a') {
		e.preventDefault();
		selection.selectAll(entries);
	} else if (mod && e.key === 'c') {
		if (selection.size > 0) {
			e.preventDefault();
			clipboard.copy(rootId, selection.paths);
			toastComponent?.show(`Copied ${selection.size} item(s)`, 'success');
		}
	} else if (mod && e.key === 'x') {
		if (selection.size > 0) {
			e.preventDefault();
			clipboard.cut(rootId, selection.paths);
			toastComponent?.show(`Cut ${selection.size} item(s)`, 'success');
		}
	} else if (mod && e.key === 'v') {
		if (clipboard.hasItems) {
			e.preventDefault();
			handlePaste();
		}
	} else if (e.key === 'F2') {
		if (selection.size === 1) {
			e.preventDefault();
			showRenameDialog = true;
		}
	}
}

async function handleDelete() {
	showDeleteConfirm = false;
	const paths = selection.paths;
	const hasDirectories = selectedEntries.some((e) => e.isDir);

	try {
		await deleteFiles(rootId, paths, hasDirectories);
		toastComponent?.show(`Deleted ${paths.length} item(s)`, 'success');
		// Also clear clipboard if deleted items were cut
		if (
			clipboard.mode === 'cut' &&
			clipboard.items.some((i) => i.rootId === rootId && paths.includes(i.path))
		) {
			clipboard.clear();
		}
		await loadDirectory(rootId, path);
	} catch (e) {
		toastComponent?.show(
			e instanceof Error ? e.message : 'Delete failed',
			'error',
		);
	}
}

async function handlePaste() {
	const op = clipboard.mode === 'cut' ? moveFiles : copyFiles;
	try {
		const results = await op({
			items: clipboard.items.map((i) => ({
				srcRoot: i.rootId,
				srcPath: i.path,
			})),
			dstRoot: rootId,
			dstPath: path || '/',
		});
		const errors = results.filter((r) => r.error);
		if (errors.length > 0) {
			toastComponent?.show(
				`${errors.length} item(s) failed to ${clipboard.mode}`,
				'error',
			);
		} else {
			toastComponent?.show(
				`${clipboard.mode === 'cut' ? 'Moved' : 'Copied'} ${results.length} item(s)`,
				'success',
			);
		}
		if (clipboard.mode === 'cut') clipboard.clear();
		await loadDirectory(rootId, path);
	} catch (e) {
		toastComponent?.show(
			e instanceof Error ? e.message : 'Paste failed',
			'error',
		);
	}
}

async function handleRename(newName: string) {
	showRenameDialog = false;
	const entry = selectedEntries[0];
	try {
		await renameFile(rootId, entry.path, newName);
		toastComponent?.show(`Renamed to "${newName}"`, 'success');
		await loadDirectory(rootId, path);
	} catch (e) {
		toastComponent?.show(
			e instanceof Error ? e.message : 'Rename failed',
			'error',
		);
	}
}

function handleDrop(e: DragEvent) {
	e.preventDefault();
	const data = e.dataTransfer?.getData('application/json');
	if (!data) return;

	try {
		const payload = JSON.parse(data) as {
			rootId: string;
			paths: string[];
			mode: 'move' | 'copy';
		};
		const op = payload.mode === 'copy' ? copyFiles : moveFiles;
		op({
			items: payload.paths.map((p: string) => ({
				srcRoot: payload.rootId,
				srcPath: p,
			})),
			dstRoot: rootId,
			dstPath: path || '/',
		}).then(async () => {
			toastComponent?.show(
				`${payload.mode === 'copy' ? 'Copied' : 'Moved'} ${payload.paths.length} item(s)`,
				'success',
			);
			await loadDirectory(rootId, path);
		});
	} catch {
		// ignore invalid drag data
	}
}

function handleDragOver(e: DragEvent) {
	e.preventDefault();
	if (e.dataTransfer) {
		e.dataTransfer.dropEffect = e.altKey ? 'copy' : 'move';
	}
}

function handleDragStartFromCard(e: DragEvent, entry: FileEntry) {
	if (!selection.has(entry.path)) {
		selection.selectOne(entry);
	}
	const paths = selection.paths;
	e.dataTransfer?.setData(
		'application/json',
		JSON.stringify({
			rootId,
			paths,
			mode: e.altKey ? 'copy' : 'move',
		}),
	);
	if (e.dataTransfer) {
		e.dataTransfer.effectAllowed = 'copyMove';
	}
}

function getContextMenuItems() {
	const hasSelection = selection.size > 0;
	return [
		{
			label: 'Open',
			icon: FolderOpen,
			action: () => {
				if (selectedEntries.length === 1) handleOpen(selectedEntries[0]);
			},
			disabled: selection.size !== 1,
		},
		{
			label: 'Rename',
			icon: Pencil,
			action: () => {
				showRenameDialog = true;
			},
			disabled: selection.size !== 1,
		},
		{
			label: 'Cut',
			icon: Scissors,
			action: () => {
				clipboard.cut(rootId, selection.paths);
				toastComponent?.show(`Cut ${selection.size} item(s)`, 'success');
			},
			disabled: !hasSelection,
		},
		{
			label: 'Copy',
			icon: Copy,
			action: () => {
				clipboard.copy(rootId, selection.paths);
				toastComponent?.show(`Copied ${selection.size} item(s)`, 'success');
			},
			disabled: !hasSelection,
		},
		{
			label: 'Paste',
			icon: ClipboardPaste,
			action: handlePaste,
			disabled: !clipboard.hasItems,
		},
		{
			label: 'Delete',
			icon: Trash2,
			action: () => {
				showDeleteConfirm = true;
			},
			disabled: !hasSelection,
			destructive: true,
		},
	];
}

function toggleFilter() {
	filterPeople = !filterPeople;
	loadDirectory(rootId, path);
}

async function handleScan() {
	try {
		scanStatus = await startScan(rootId, path);
		toastComponent?.show(`Scanning ${scanStatus.total} images...`, 'success');
		startPollingScanStatus();
	} catch (e) {
		toastComponent?.show(
			e instanceof Error ? e.message : 'Scan failed',
			'error',
		);
	}
}

function startPollingScanStatus() {
	if (scanPollTimer) clearInterval(scanPollTimer);
	scanPollTimer = setInterval(async () => {
		try {
			scanStatus = await getScanStatus();
			if (!scanStatus.running) {
				if (scanPollTimer) clearInterval(scanPollTimer);
				scanPollTimer = null;
				toastComponent?.show(
					`Scan complete: ${scanStatus.completed} images processed`,
					'success',
				);
				// Reload to reflect new detection badges (and filter if active)
				loadDirectory(rootId, path);
			}
		} catch {
			if (scanPollTimer) clearInterval(scanPollTimer);
			scanPollTimer = null;
		}
	}, 1000);
}

function formatDate(dateStr: string): string {
	try {
		return new Date(dateStr).toISOString().replace('T', ' ').slice(0, 19);
	} catch {
		return dateStr;
	}
}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if previewIndex !== -1 && previewFile}
	<MediaPreview
		{rootId}
		{mediaEntries}
		currentIndex={previewIndex}
		onClose={() => onPreviewChange?.(null)}
		onIndexChange={(idx) => onPreviewChange?.(mediaEntries[idx].path)}
	/>
{:else}
	<div class="flex h-full flex-col bg-gray-950">
		<!-- Toolbar -->
		<div class="flex items-center justify-between border-b border-gray-800 bg-gray-900 px-4 py-2">
			<div class="flex items-center gap-4 min-w-0 flex-1">
				<div class="truncate text-sm font-medium text-gray-300">
					<button
						class="text-gray-500 hover:text-gray-200 transition-colors"
						onclick={() => onNavigate?.('')}
					>
						{rootId}
					</button>
					{#if path}
						{@const segments = path.split('/')}
						{#each segments as segment, i}
							<span class="mx-1 text-gray-600">/</span>
							{#if i < segments.length - 1}
								<button
									class="text-gray-400 hover:text-gray-200 transition-colors"
									onclick={() => onNavigate?.(segments.slice(0, i + 1).join('/'))}
								>
									{segment}
								</button>
							{:else}
								<span>{segment}</span>
							{/if}
						{/each}
					{:else}
						<span class="mx-1 text-gray-600">/</span>
						<span>(root)</span>
					{/if}
				</div>

				<!-- File operation buttons -->
				<div class="flex items-center gap-1">
					<button
						class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
						title="Cut (Ctrl+X)"
						disabled={selection.size === 0}
						onclick={() => {
							clipboard.cut(rootId, selection.paths);
							toastComponent?.show(`Cut ${selection.size} item(s)`, 'success');
						}}
					>
						<Scissors size={16} />
					</button>
					<button
						class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
						title="Copy (Ctrl+C)"
						disabled={selection.size === 0}
						onclick={() => {
							clipboard.copy(rootId, selection.paths);
							toastComponent?.show(`Copied ${selection.size} item(s)`, 'success');
						}}
					>
						<Copy size={16} />
					</button>
					<button
						class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
						title="Paste (Ctrl+V)"
						disabled={!clipboard.hasItems}
						onclick={handlePaste}
					>
						<ClipboardPaste size={16} />
					</button>
					<button
						class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
						title="Rename (F2)"
						disabled={selection.size !== 1}
						onclick={() => (showRenameDialog = true)}
					>
						<Pencil size={16} />
					</button>
					<button
						class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-red-400 disabled:opacity-30 disabled:cursor-not-allowed"
						title="Delete"
						disabled={selection.size === 0}
						onclick={() => (showDeleteConfirm = true)}
					>
						<Trash2 size={16} />
					</button>

					<div class="mx-1 h-4 w-px bg-gray-700"></div>

					<button
						class="rounded p-1.5 transition-colors {filterPeople ? 'bg-blue-600 text-white' : 'text-gray-500 hover:bg-gray-800 hover:text-gray-300'}"
						title="Hide images with people"
						onclick={toggleFilter}
					>
						<UserX size={16} />
					</button>
					<button
						class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
						title="Scan for people"
						disabled={scanStatus?.running === true}
						onclick={handleScan}
					>
						<ScanSearch size={16} />
					</button>
				</div>
			</div>

			<div class="flex items-center gap-2">
				{#if scanStatus?.running}
					<span class="text-xs text-blue-400">
						Scanning {scanStatus.completed}/{scanStatus.total}
					</span>
				{/if}
				{#if selection.size > 0}
					<span class="text-xs text-gray-500">{selection.size} selected</span>
				{/if}
				<div class="flex items-center gap-1 rounded-lg bg-gray-800 p-1">
					<button
						class="rounded px-2 py-1 text-xs font-medium transition-colors {thumbSize === 'small' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-200'}"
						onclick={() => (thumbSize = 'small')}
					>
						Small
					</button>
					<button
						class="rounded px-2 py-1 text-xs font-medium transition-colors {thumbSize === 'medium' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-200'}"
						onclick={() => (thumbSize = 'medium')}
					>
						Medium
					</button>
					<button
						class="rounded px-2 py-1 text-xs font-medium transition-colors {thumbSize === 'large' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-200'}"
						onclick={() => (thumbSize = 'large')}
					>
						Large
					</button>
				</div>
			</div>
		</div>

		<div class="flex flex-1 overflow-hidden">
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="flex-1 overflow-y-auto p-4"
				bind:this={scrollContainer}
				onclick={() => selection.clear()}
				ondrop={handleDrop}
				ondragover={handleDragOver}
			>
				{#if loading}
					<div class="flex h-full items-center justify-center">
						<p class="text-gray-500">Loading...</p>
					</div>
				{:else if error}
					<div class="flex h-full items-center justify-center">
						<p class="text-red-400">{error}</p>
					</div>
				{:else if entries.length === 0}
					<div class="flex h-full items-center justify-center">
						<p class="text-gray-500">Empty directory</p>
					</div>
				{:else}
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div class={`grid gap-3 ${gridClass}`} onclick={(e) => e.stopPropagation()}>
						{#each entries as entry (entry.path)}
							<ThumbnailCard
								{rootId}
								{entry}
								selected={selection.has(entry.path)}
								cut={clipboard.isCut(rootId, entry.path)}
								onclick={(e) => handleClick(entry, e)}
								ondblclick={() => handleOpen(entry)}
								oncontextmenu={(e) => handleContextMenu(e, entry)}
								ondragstart={(e) => handleDragStartFromCard(e, entry)}
							/>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Details Panel -->
			<div class="w-80 overflow-y-auto border-l border-gray-800 bg-gray-900 p-6 shadow-xl">
				{#if selectedEntries.length === 1}
					{@const sel = selectedEntries[0]}
					<div class="flex flex-col gap-6">
						<div class="aspect-video w-full overflow-hidden rounded-lg bg-gray-950 shadow-inner">
							{#if sel.hasThumb}
								<img
									src={thumbnailUrl(rootId, sel.path)}
									alt={sel.name}
									class="h-full w-full object-contain"
								/>
							{:else}
								<div class="flex h-full items-center justify-center text-gray-700">
									<FileIcon entry={sel} size={64} />
								</div>
							{/if}
						</div>

						<div class="space-y-4">
							<div>
								<h3 class="break-all text-lg font-semibold text-gray-100">{sel.name}</h3>
								<p class="text-sm text-gray-400">{sel.mediaType}</p>
							</div>

							<div class="grid grid-cols-2 gap-y-4 text-sm">
								<div class="text-gray-500">Size</div>
								<div class="text-gray-300">{formatSize(sel.size)}</div>

								<div class="text-gray-500">Modified</div>
								<div class="text-gray-300">{formatDate(sel.modTime)}</div>

								<div class="text-gray-500">Path</div>
								<div class="break-all text-gray-300">{sel.path}</div>

								{#if detectionResult}
									<div class="text-gray-500">Person</div>
									<div class="text-gray-300">
										{#if detectionResult.hasPerson}
											<span class="text-red-400">Yes ({(detectionResult.confidence * 100).toFixed(0)}%)</span>
										{:else}
											<span class="text-green-400">No</span>
										{/if}
									</div>
								{/if}
							</div>
						</div>

						<div class="mt-auto pt-6">
							<button
								class="w-full rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 focus:ring-offset-gray-900"
								onclick={() => handleOpen(sel)}
							>
								{sel.isDir ? 'Open Folder' : 'Open'}
							</button>
						</div>
					</div>
				{:else if selectedEntries.length > 1}
					<div class="flex flex-col gap-4">
						<h3 class="text-lg font-semibold text-gray-100">{selectedEntries.length} items selected</h3>
						<div class="grid grid-cols-2 gap-y-4 text-sm">
							<div class="text-gray-500">Total size</div>
							<div class="text-gray-300">{formatSize(selectedEntries.reduce((sum, e) => sum + e.size, 0))}</div>

							<div class="text-gray-500">Files</div>
							<div class="text-gray-300">{selectedEntries.filter((e) => !e.isDir).length}</div>

							<div class="text-gray-500">Folders</div>
							<div class="text-gray-300">{selectedEntries.filter((e) => e.isDir).length}</div>
						</div>
					</div>
				{:else}
					<div class="flex h-full items-center justify-center">
						<p class="text-center text-sm text-gray-600">Select a file to view details</p>
					</div>
				{/if}
			</div>
		</div>
	</div>
{/if}

{#if showDeleteConfirm}
	<ConfirmDialog
		title="Delete {selectedEntries.length} item(s)?"
		items={selectedEntries.map((e) => e.name)}
		onConfirm={handleDelete}
		onCancel={() => (showDeleteConfirm = false)}
	/>
{/if}

{#if contextMenu}
	<ContextMenu
		x={contextMenu.x}
		y={contextMenu.y}
		items={getContextMenuItems()}
		onClose={() => (contextMenu = null)}
	/>
{/if}

{#if showRenameDialog && selectedEntries.length === 1}
	<RenameDialog
		currentName={selectedEntries[0].name}
		onConfirm={handleRename}
		onCancel={() => (showRenameDialog = false)}
	/>
{/if}

<Toast bind:this={toastComponent} />

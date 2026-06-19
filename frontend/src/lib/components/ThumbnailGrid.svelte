<script lang="ts">
import {
	deleteFiles,
	fetchDirectory,
	moveFiles,
	copyFiles,
	renameFile,
	startScan,
	getScanStatus,
	startClassifyScan,
	getClassifyScanStatus,
	startOCRScan,
	getOCRScanStatus,
	startAnalyzeScan,
	getAnalyzeScanStatus,
	searchFiles,
	getAnalysisSettings,
	setAnalysisSettings,
	getCachedDirectory,
	invalidateDirectoryCache,
	invalidateParentDirectoryCache,
	fileUrl,
} from '$lib/api';
import type { AnalysisSettings } from '$lib/api';
import type { FileEntry } from '$lib/types';
import { sortEntries } from '$lib/utils';
import { createSelection } from '$lib/selection.svelte';
import { createClipboard } from '$lib/clipboard.svelte';
import { createScan, makeReloadAfterScan } from '$lib/scans.svelte';
import MediaPreview from './MediaPreview.svelte';
import ThumbnailCard from './ThumbnailCard.svelte';
import ListView from './ListView.svelte';
import ConfirmDialog from './ConfirmDialog.svelte';
import RenameDialog from './RenameDialog.svelte';
import ContextMenu from './ContextMenu.svelte';
import Toast from './Toast.svelte';
import GridToolbar from './GridToolbar.svelte';
import DetailsPanel from './DetailsPanel.svelte';
import DuplicatesModal from './DuplicatesModal.svelte';
import {
	Trash2,
	Scissors,
	Copy,
	ClipboardPaste,
	FolderOpen,
	Pencil,
	LoaderCircle,
} from '@lucide/svelte';

interface Props {
	rootId: string;
	rootName?: string;
	path: string;
	previewFile?: string | null;
	onNavigate?: (path: string) => void;
	onPreviewChange?: (path: string | null) => void;
}

let {
	rootId,
	rootName,
	path,
	previewFile = null,
	onNavigate,
	onPreviewChange,
}: Props = $props();

let pathSegments = $derived(path.split('/').filter(Boolean));
let entries = $state<FileEntry[]>([]);
let loading = $state(false);
let backgroundValidating = $state(false);
let loadingSlow = $state(false);
let loadingSlowTimer: ReturnType<typeof setTimeout> | null = null;
let error = $state<string | null>(null);
let thumbSize = $state<'small' | 'medium' | 'large'>('medium');
let savedScrollTop = $state(0);
let scrollContainer: HTMLDivElement | undefined = $state();
let viewMode = $state<'grid' | 'list'>('grid');

const selection = createSelection();
const clipboard = createClipboard();

let showDeleteConfirm = $state(false);
let showRenameDialog = $state(false);
let contextMenu = $state<{ x: number; y: number } | null>(null);
let toastComponent: Toast | undefined = $state();
let filterPeople = $state(false);
let filterTag = $state<string>('');
let analysisSettings = $state<AnalysisSettings | null>(null);
let analysisSettingsSaving = $state(false);
let analysisPollTimer: ReturnType<typeof setInterval> | null = null;
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null;
let loadRequestId = 0;
let latestVisibleLoadId = 0;

// Search state
let searchActive = $state(false);
let searchQuery = $state('');
let searchResults = $state<FileEntry[]>([]);
let searchLoading = $state(false);
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null;
let searchInput: HTMLInputElement | undefined = $state();

// Duplicates state
let showDuplicates = $state(false);

const toast = (msg: string, kind: 'success' | 'error') =>
	toastComponent?.show(msg, kind);
const reloadAfterScan = (invalidate: boolean) =>
	makeReloadAfterScan(
		{
			invalidate: invalidateDirectoryCache,
			reload: loadDirectory,
			current: () => ({ rootId, path }),
		},
		invalidate,
	);

// Background scans share one start → poll → reload flow (see scans.svelte.ts).
const detectScan = createScan({
	start: startScan,
	poll: getScanStatus,
	startMsg: (s) => `Scanning ${s.total} images...`,
	doneMsg: (s) => `Scan complete: ${s.completed} images processed`,
	errMsg: 'Scan failed',
	onToast: toast,
	onComplete: reloadAfterScan(true),
});
const classifyScan = createScan({
	start: startClassifyScan,
	poll: getClassifyScanStatus,
	startMsg: (s) => `Classifying ${s.total} images...`,
	doneMsg: (s) => `Classification complete: ${s.completed} images processed`,
	errMsg: 'Classification scan failed',
	onToast: toast,
	onComplete: reloadAfterScan(true),
});
const ocrScan = createScan({
	start: startOCRScan,
	poll: getOCRScanStatus,
	startMsg: (s) => `Running OCR on ${s.total} files...`,
	doneMsg: (s) => `OCR complete: ${s.completed} files processed`,
	errMsg: 'OCR scan failed',
	onToast: toast,
	onComplete: reloadAfterScan(false),
});
const analyzeScan = createScan({
	start: (rid, p) => startAnalyzeScan(rid, p, true),
	poll: getAnalyzeScanStatus,
	startMsg: (s) => `Re-analyzing ${s.total} files...`,
	doneMsg: (s) => `Analysis complete: ${s.completed} files processed`,
	errMsg: 'Re-analysis failed',
	onToast: toast,
	onComplete: reloadAfterScan(false),
});

let availableTags = $derived.by(() => {
	const tagSet = new Set<string>();
	for (const e of displayEntries) {
		if (e.tags) {
			for (const t of e.tags) {
				tagSet.add(t.label);
			}
		}
	}
	return Array.from(tagSet).sort();
});

let displayEntries = $derived(
	searchActive && searchQuery ? searchResults : entries,
);

let mediaEntries = $derived(
	displayEntries.filter(
		(e) => e.mediaType === 'image' || e.mediaType === 'video',
	),
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

let selectedEntries = $derived(
	displayEntries.filter((e) => selection.has(e.path)),
);

$effect(() => {
	closeSearch();
	loadDirectory(rootId, path);
});

$effect(() => {
	loadAnalysisSettings();
	if (analysisPollTimer) {
		clearInterval(analysisPollTimer);
	}
	analysisPollTimer = setInterval(() => {
		void loadAnalysisSettings();
	}, 2000);
});

$effect(() => {
	return () => {
		detectScan.dispose();
		classifyScan.dispose();
		ocrScan.dispose();
		analyzeScan.dispose();
		if (autoRefreshTimer) {
			clearInterval(autoRefreshTimer);
			autoRefreshTimer = null;
		}
		if (searchDebounceTimer) {
			clearTimeout(searchDebounceTimer);
			searchDebounceTimer = null;
		}
		if (analysisPollTimer) {
			clearInterval(analysisPollTimer);
			analysisPollTimer = null;
		}
		if (loadingSlowTimer) {
			clearTimeout(loadingSlowTimer);
			loadingSlowTimer = null;
		}
	};
});

$effect(() => {
	if (autoRefreshTimer) {
		clearInterval(autoRefreshTimer);
		autoRefreshTimer = null;
	}

	if (!analysisSettings?.autoBrowseEnabled) {
		return;
	}

	autoRefreshTimer = setInterval(() => {
		if (
			loading ||
			searchLoading ||
			detectScan.running ||
			classifyScan.running ||
			ocrScan.running ||
			analyzeScan.running ||
			(searchActive && !!searchQuery.trim())
		) {
			return;
		}
		// Only refresh while background analysis is actually producing new
		// results. Idle folders (e.g. no images to analyze) would otherwise
		// re-list the directory forever.
		const browse = analysisSettings?.browseStatus;
		if (!browse || (!browse.running && browse.queued === 0)) {
			return;
		}
		loadDirectory(rootId, path, { preserveSelection: true, silent: true });
	}, 2000);

	return () => {
		if (autoRefreshTimer) {
			clearInterval(autoRefreshTimer);
			autoRefreshTimer = null;
		}
	};
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

async function loadAnalysisSettings() {
	try {
		analysisSettings = await getAnalysisSettings();
	} catch {
		analysisSettings = null;
	}
}

async function loadDirectory(
	rid: string,
	p: string,
	options?: { preserveSelection?: boolean; silent?: boolean },
) {
	const preserveSelection = options?.preserveSelection ?? false;
	const silent = options?.silent ?? false;
	const requestId = ++loadRequestId;

	if (!preserveSelection) {
		selection.clear();
		savedScrollTop = 0;
	}

	const filterOpts: { filter?: string; tag?: string } = {};
	if (filterPeople) filterOpts.filter = 'no-people';
	if (filterTag) filterOpts.tag = filterTag;

	// Check if cached entries exist
	const cached = getCachedDirectory(rid, p, filterOpts);
	if (cached) {
		entries = sortEntries(cached);
	}

	// Determine if we show the fullscreen loading skeleton
	const showVisualLoader = !silent && !cached;

	if (showVisualLoader) {
		latestVisibleLoadId = requestId;
		loading = true;
		loadingSlow = false;
		if (loadingSlowTimer) clearTimeout(loadingSlowTimer);
		loadingSlowTimer = setTimeout(() => {
			loadingSlow = true;
		}, 3000);
	} else if (!silent) {
		backgroundValidating = true;
	}

	error = null;
	try {
		const result = await fetchDirectory(
			rid,
			p,
			Object.keys(filterOpts).length > 0 ? filterOpts : undefined,
		);
		if (requestId !== loadRequestId) return;
		entries = sortEntries(result);
	} catch (e) {
		if (requestId !== loadRequestId) return;
		if (entries.length === 0) {
			error = e instanceof Error ? e.message : 'Failed to load directory';
			entries = [];
		} else {
			toastComponent?.show(
				e instanceof Error
					? `Failed to refresh: ${e.message}`
					: 'Failed to refresh folder contents',
				'error',
			);
		}
	} finally {
		if (!silent && requestId === latestVisibleLoadId) {
			loading = false;
			loadingSlow = false;
			if (loadingSlowTimer) {
				clearTimeout(loadingSlowTimer);
				loadingSlowTimer = null;
			}
		}
		backgroundValidating = false;
	}
}

function handleSearchInput(e: Event) {
	const value = (e.target as HTMLInputElement).value;
	searchQuery = value;

	if (searchDebounceTimer) clearTimeout(searchDebounceTimer);

	if (!value.trim()) {
		searchResults = [];
		return;
	}

	searchDebounceTimer = setTimeout(async () => {
		searchLoading = true;
		try {
			const results = await searchFiles(rootId, value, path || undefined);
			searchResults = results;
		} catch {
			searchResults = [];
		}
		searchLoading = false;
	}, 300);
}

function openSearch() {
	searchActive = true;
	requestAnimationFrame(() => searchInput?.focus());
}

function closeSearch() {
	searchActive = false;
	searchQuery = '';
	searchResults = [];
	if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
}

function handleClick(entry: FileEntry, e: MouseEvent) {
	if (e.metaKey || e.ctrlKey) {
		selection.toggle(entry);
	} else if (e.shiftKey) {
		selection.selectRange(displayEntries, entry);
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
	} else if (entry.mediaType === 'pdf') {
		window.open(fileUrl(rootId, entry.path), '_blank');
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

	if (mod && e.key === 'f') {
		e.preventDefault();
		if (searchActive) {
			searchInput?.focus();
		} else {
			openSearch();
		}
		return;
	}

	if (e.key === 'Escape' && searchActive) {
		closeSearch();
		return;
	}

	if (e.key === 'Delete' || (e.key === 'Backspace' && mod)) {
		if (selection.size > 0) {
			e.preventDefault();
			showDeleteConfirm = true;
		}
	} else if (mod && e.key === 'a') {
		e.preventDefault();
		selection.selectAll(displayEntries);
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
		invalidateDirectoryCache(rootId, path);
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
		if (clipboard.mode === 'cut') {
			for (const item of clipboard.items) {
				invalidateParentDirectoryCache(item.rootId, item.path);
			}
			clipboard.clear();
		}
		invalidateDirectoryCache(rootId, path);
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
		invalidateDirectoryCache(rootId, path);
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
			if (payload.mode === 'move') {
				for (const p of payload.paths) {
					invalidateParentDirectoryCache(payload.rootId, p);
				}
			}
			invalidateDirectoryCache(rootId, path);
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

async function toggleAutoBrowseAnalysis() {
	if (!analysisSettings || analysisSettingsSaving) return;
	const next = !analysisSettings.autoBrowseEnabled;
	analysisSettingsSaving = true;
	try {
		analysisSettings = await setAnalysisSettings(next);
		toastComponent?.show(
			next
				? 'Auto analyze while browsing enabled'
				: 'Auto analyze while browsing disabled',
			'success',
		);
	} catch {
		toastComponent?.show('Failed to update auto analysis setting', 'error');
	} finally {
		analysisSettingsSaving = false;
	}
}

// Re-analyze: one pass over the folder that loads each file once and runs every
// enabled analyzer (detection + tags + OCR), replacing all existing results.
function handleReanalyzeAll() {
	if (
		!confirm(
			'Re-analyze every image in this folder and its subfolders from scratch? This runs detection, tagging, and OCR, replacing all existing results, and may take a while.',
		)
	) {
		return;
	}
	analyzeScan.run(rootId, path);
}

function handleTagFilter(e: Event) {
	const target = e.target as HTMLSelectElement;
	filterTag = target.value;
	loadDirectory(rootId, path);
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
		<GridToolbar
			{rootId}
			{rootName}
			{pathSegments}
			{backgroundValidating}
			selectionSize={selection.size}
			hasClipboard={clipboard.hasItems}
			{searchActive}
			{searchQuery}
			{searchLoading}
			searchResultCount={searchResults.length}
			{analysisSettings}
			{analysisSettingsSaving}
			scanStatus={detectScan.status}
			classifyScanStatus={classifyScan.status}
			ocrScanStatus={ocrScan.status}
			analyzeScanStatus={analyzeScan.status}
			{filterPeople}
			{filterTag}
			{availableTags}
			{viewMode}
			{thumbSize}
			bind:searchInput
			onNavigate={(p) => onNavigate?.(p)}
			onOpenSearch={openSearch}
			onCloseSearch={closeSearch}
			onSearchInput={handleSearchInput}
			onCut={() => {
				clipboard.cut(rootId, selection.paths);
				toastComponent?.show(`Cut ${selection.size} item(s)`, 'success');
			}}
			onCopy={() => {
				clipboard.copy(rootId, selection.paths);
				toastComponent?.show(`Copied ${selection.size} item(s)`, 'success');
			}}
			onPaste={handlePaste}
			onRename={() => (showRenameDialog = true)}
			onDelete={() => (showDeleteConfirm = true)}
			onToggleAutoBrowse={toggleAutoBrowseAnalysis}
			onToggleFilter={toggleFilter}
			onScan={() => detectScan.run(rootId, path)}
			onClassify={() => classifyScan.run(rootId, path)}
			onOCR={() => ocrScan.run(rootId, path)}
			onReanalyze={handleReanalyzeAll}
			onFindDuplicates={() => (showDuplicates = true)}
			onTagFilter={handleTagFilter}
			onViewMode={(m) => (viewMode = m)}
			onThumbSize={(s) => (thumbSize = s)}
		/>

		<div class="flex flex-1 overflow-hidden">
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="flex-1 overflow-y-auto {viewMode === 'grid' ? 'p-4' : ''}"
				bind:this={scrollContainer}
				onclick={() => selection.clear()}
				ondrop={handleDrop}
				ondragover={handleDragOver}
			>
				{#if loading || (searchActive && searchLoading)}
					<div class="flex h-full flex-col items-center justify-center gap-4 px-4">
						<div class="flex flex-wrap justify-center gap-3">
							{#each Array(8)}
								<div class="h-32 w-44 animate-pulse rounded-lg bg-gray-800"></div>
							{/each}
						</div>
						<div class="text-center">
							<p class="text-gray-500">
								<LoaderCircle size={16} class="mr-2 inline animate-spin" />
								Loading...
							</p>
							{#if loadingSlow}
								<p class="mt-1 text-sm text-amber-400">Still loading — network drives may respond slowly</p>
							{/if}
						</div>
					</div>
				{:else if error}
					<div class="flex h-full items-center justify-center">
						<p class="text-red-400">{error}</p>
					</div>
				{:else if displayEntries.length === 0}
					<div class="flex h-full items-center justify-center">
						<p class="text-gray-500">{searchActive && searchQuery ? 'No results found' : 'Empty directory'}</p>
					</div>
				{:else if viewMode === 'list'}
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div onclick={(e) => e.stopPropagation()}>
						<ListView
							{rootId}
							entries={displayEntries}
							isSelected={(path) => selection.has(path)}
							isCut={(path) => clipboard.isCut(rootId, path)}
							onclick={(entry, e) => handleClick(entry, e)}
							ondblclick={(entry) => handleOpen(entry)}
							oncontextmenu={(e, entry) => handleContextMenu(e, entry)}
							ondragstart={(e, entry) => handleDragStartFromCard(e, entry)}
						/>
					</div>
				{:else}
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div class={`grid gap-3 ${gridClass}`} onclick={(e) => e.stopPropagation()}>
						{#each displayEntries as entry (entry.path)}
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

			<DetailsPanel
				{rootId}
				{selectedEntries}
				onOpen={handleOpen}
				onToast={toast}
			/>
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

{#if showDuplicates}
	<DuplicatesModal
		{rootId}
		{path}
		onClose={() => (showDuplicates = false)}
	/>
{/if}

<Toast bind:this={toastComponent} />

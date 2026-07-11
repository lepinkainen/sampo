<script lang="ts">
import { onDestroy } from 'svelte';
import {
	getClassification,
	getDetection,
	getDiskUsage,
	runOCR,
} from '$lib/api';
import type {
	ClassificationResult,
	DetectionResult,
	DiskUsage,
	OCRResult,
} from '$lib/api';
import { ThumbnailLoader, thumbnailKey } from '$lib/thumbnailLoader.svelte';
import { thumbnailQueue } from '$lib/thumbnailQueue';
import type { FileEntry } from '$lib/types';
import {
	formatDate,
	formatDuration,
	formatResolution,
	formatSize,
} from '$lib/utils';
import FileIcon from './FileIcon.svelte';
import Loader from './Loader.svelte';

interface Props {
	rootId: string;
	selectedEntries: FileEntry[];
	onOpen: (entry: FileEntry) => void;
	onToast: (msg: string, kind: 'success' | 'error') => void;
}

let { rootId, selectedEntries, onOpen, onToast }: Props = $props();

let sel = $derived(selectedEntries.length === 1 ? selectedEntries[0] : null);

let detectionResult = $state<DetectionResult | null>(null);
let classificationResult = $state<ClassificationResult | null>(null);
let ocrResult = $state<OCRResult | null>(null);
let ocrLoading = $state(false);
let ocrError = $state<string | null>(null);
let diskUsage = $state<DiskUsage | null>(null);
let diskUsageLoading = $state(false);

const detailsThumb = new ThumbnailLoader();
let lastDetailsThumbKey = $state('');
let lastSelectionDetailsKey = $state('');
// Priority 0: an explicitly-selected file should preempt every grid
// thumbnail, including cheap images. Only one entry is alive at a time.
let detailsThumbCancel: (() => void) | null = null;

let selectedDetailsTags = $derived.by(() => {
	if (!sel) return [];
	if (
		classificationResult?.rootId === rootId &&
		classificationResult.relPath === sel.path
	) {
		return classificationResult.tags;
	}
	return sel.tags ?? [];
});

onDestroy(() => {
	detailsThumbCancel?.();
	detailsThumb.cleanup();
});

$effect(() => {
	const thumbKey = sel ? thumbnailKey(rootId, sel) : '';
	if (thumbKey === lastDetailsThumbKey) {
		return;
	}
	lastDetailsThumbKey = thumbKey;
	detailsThumbCancel?.();
	detailsThumbCancel = null;
	detailsThumb.reset(sel?.hasThumb ? 'loading' : 'idle');
	if (sel?.hasThumb) {
		// Route through the shared queue so the panel's single thumbnail
		// preempts any in-flight grid fetches (priority 0 = lowest cost).
		detailsThumbCancel = thumbnailQueue.enqueue(0, () =>
			detailsThumb.fetch(rootId, sel.path),
		);
	}
});

$effect(() => {
	const selectionKey = sel
		? `${rootId}:${sel.path}:${sel.isDir}:${sel.mediaType}`
		: '';
	if (selectionKey === lastSelectionDetailsKey) {
		return;
	}
	lastSelectionDetailsKey = selectionKey;

	detectionResult = null;
	classificationResult = null;
	ocrResult = null;
	ocrError = null;
	diskUsage = null;
	if (!sel) {
		return;
	}

	// Seed OCR display from cached listing text (no recompute on select).
	if (sel.ocrText) {
		ocrResult = {
			rootId,
			relPath: sel.path,
			text: sel.ocrText,
			blocks: [],
			modelVer: '',
			scannedAt: '',
		};
	}
	if (sel.mediaType === 'image') {
		getDetection(rootId, sel.path)
			.then((r) => (detectionResult = r))
			.catch(() => (detectionResult = null));
		getClassification(rootId, sel.path)
			.then((r) => (classificationResult = r))
			.catch(() => (classificationResult = null));
	}
	if (sel.isDir) {
		diskUsageLoading = true;
		getDiskUsage(rootId, sel.path)
			.then((r) => (diskUsage = r))
			.catch(() => (diskUsage = null))
			.finally(() => (diskUsageLoading = false));
	}
});

// Re-seed OCR display if text arrives after selection (e.g. background
// analysis updates the entry while it stays selected).
$effect(() => {
	if (!sel?.ocrText || ocrResult) {
		return;
	}
	ocrResult = {
		rootId,
		relPath: sel.path,
		text: sel.ocrText,
		blocks: [],
		modelVer: '',
		scannedAt: '',
	};
});

async function handleRunOCR(entry: FileEntry) {
	ocrLoading = true;
	ocrError = null;
	try {
		const force = Boolean(ocrResult || entry.ocrText);
		ocrResult = await runOCR(rootId, entry.path, force);
		// Reflect the result on the cached entry so it persists across reselects.
		entry.ocrText = ocrResult.text;
		if (!ocrResult.text) {
			onToast('No text found in image', 'success');
		}
	} catch (e) {
		ocrError = e instanceof Error ? e.message : 'OCR failed';
		onToast(ocrError, 'error');
	} finally {
		ocrLoading = false;
	}
}
</script>

<div class="w-80 overflow-y-auto border-l border-raised bg-surface p-6 shadow-xl">
	{#if sel}
		<div class="flex flex-col gap-6">
			<div class="relative aspect-video w-full overflow-hidden rounded-lg bg-canvas shadow-inner">
				{#if sel.hasThumb && detailsThumb.state === 'ready' && detailsThumb.objectUrl}
					<img
						src={detailsThumb.objectUrl}
						alt={sel.name}
						class="relative z-0 h-full w-full object-contain"
						onerror={() => detailsThumb.fail()}
					/>
				{:else if sel.hasThumb && detailsThumb.state !== 'error'}
					<div
						class="thumb-skeleton absolute inset-0 z-10"
						data-testid="details-thumbnail-skeleton"
					></div>
					{#if detailsThumb.showSlowLoading}
						<div
							class="absolute inset-0 z-20 flex items-center justify-center text-faint"
							aria-label="Loading thumbnail"
						>
							<Loader />
						</div>
					{/if}
				{:else}
					<div class="flex h-full items-center justify-center text-muted">
						<FileIcon entry={sel} size={64} />
					</div>
				{/if}
			</div>

			<div class="space-y-4">
				<div>
					<h3 class="break-all text-lg font-semibold text-bright">{sel.name}</h3>
					<p class="text-sm text-dim">{sel.mediaType}</p>
				</div>

				<div class="grid grid-cols-2 gap-y-4 text-sm">
				{#if !sel.isDir}
					<div class="text-faint">Size</div>
					<div class="text-body">{formatSize(sel.size)}</div>

					{#if sel.width && sel.height}
						<div class="text-faint">Resolution</div>
						<div class="text-body">{formatResolution(sel.width, sel.height, sel.mediaType)}</div>
					{/if}

					{#if sel.duration}
						<div class="text-faint">Duration</div>
						<div class="text-body">{formatDuration(sel.duration)}</div>
					{/if}
				{/if}

					<div class="text-faint">Modified</div>
					<div class="text-body">{formatDate(sel.modTime)}</div>

					<div class="text-faint">Path</div>
					<div class="break-all text-body">{sel.path}</div>

					{#if sel.isDir && diskUsageLoading}
						<div class="text-faint">Usage</div>
						<div class="text-dim">Computing...</div>
					{/if}

					{#if sel.isDir && diskUsage}
						<div class="text-faint">Total size</div>
						<div class="text-body">{formatSize(diskUsage.totalSize)}</div>

						<div class="text-faint">Files</div>
						<div class="text-body">{diskUsage.fileCount}</div>

						<div class="text-faint">Subdirs</div>
						<div class="text-body">{diskUsage.dirCount}</div>
					{/if}

					{#if detectionResult}
						<div class="text-faint">Person</div>
						<div class="text-body">
							{#if detectionResult.hasPerson}
								<span class="text-danger-soft">Yes ({(detectionResult.confidence * 100).toFixed(0)}%)</span>
							{:else}
								<span class="text-ok">No</span>
							{/if}
						</div>
					{/if}

					{#if sel.sha256}
						<div class="text-faint">SHA256</div>
						<div class="text-body">
							<button
								class="font-mono text-xs break-all text-left hover:text-accent-soft transition-colors"
								title="Click to copy full hash"
								onclick={() => { navigator.clipboard.writeText(sel.sha256 ?? ''); onToast('SHA256 copied', 'success'); }}
							>
								{sel.sha256.slice(0, 16)}...
							</button>
						</div>
					{/if}

					{#if sel.crc32}
						<div class="text-faint">CRC32</div>
						<div class="text-body font-mono text-xs">{sel.crc32}</div>
					{/if}

					{#if selectedDetailsTags.length > 0}
						<div class="col-span-2 border-t border-raised pt-2">
							<div class="text-faint mb-1">Tags</div>
							<div class="flex flex-wrap gap-1">
								{#each selectedDetailsTags as tag}
									<span class="rounded bg-tag/80 px-1.5 py-0.5 text-xs text-white" title={`${(tag.score * 100).toFixed(0)}%`}>
										{tag.label} <span class="text-tag-faint">{(tag.score * 100).toFixed(0)}%</span>
									</span>
								{/each}
							</div>
						</div>
					{/if}

					{#if !sel.isDir && (sel.mediaType === 'image' || sel.mediaType === 'video')}
						<div class="col-span-2 border-t border-raised pt-2">
							<div class="mb-1 flex items-center justify-between">
								<span class="text-faint">OCR text</span>
								<button
									class="rounded bg-muted px-2 py-0.5 text-xs text-body hover:bg-ghost disabled:cursor-not-allowed disabled:opacity-50"
									onclick={() => handleRunOCR(sel)}
									disabled={ocrLoading}
									title="Extract text from this image"
								>
									{ocrLoading ? 'Running…' : ocrResult ? 'Re-run OCR' : 'Run OCR'}
								</button>
							</div>
							{#if ocrError}
								<p class="text-xs text-danger-soft">{ocrError}</p>
							{:else if ocrResult && ocrResult.blocks.length > 0}
								<div class="flex flex-col gap-0.5">
									{#each ocrResult.blocks as block}
										<span class="rounded bg-raised px-1.5 py-0.5 text-xs break-all text-body">{block.text}</span>
									{/each}
								</div>
							{:else if ocrResult?.text}
								<p class="text-xs break-words whitespace-pre-wrap text-body">{ocrResult.text}</p>
							{:else if ocrResult}
								<p class="text-xs text-faint">No text found</p>
							{:else}
								<p class="text-xs text-ghost">Not analyzed yet</p>
							{/if}
						</div>
					{/if}
				</div>
			</div>

			<div class="mt-auto pt-6">
				<button
					class="w-full rounded-md bg-accent px-4 py-2 text-sm font-medium text-on-accent hover:bg-accent-hover focus:outline-none focus:ring-2 focus:ring-accent-hover focus:ring-offset-2 focus:ring-offset-surface"
					onclick={() => onOpen(sel)}
				>
					{sel.isDir ? 'Open Folder' : 'Open'}
				</button>
			</div>
		</div>
	{:else if selectedEntries.length > 1}
		<div class="flex flex-col gap-4">
			<h3 class="text-lg font-semibold text-bright">{selectedEntries.length} items selected</h3>
			<div class="grid grid-cols-2 gap-y-4 text-sm">
				<div class="text-faint">Total size</div>
				<div class="text-body">{formatSize(selectedEntries.reduce((sum, e) => (e.isDir ? sum : sum + e.size), 0))}</div>

				<div class="text-faint">Files</div>
				<div class="text-body">{selectedEntries.filter((e) => !e.isDir).length}</div>

				<div class="text-faint">Folders</div>
				<div class="text-body">{selectedEntries.filter((e) => e.isDir).length}</div>
			</div>
		</div>
	{:else}
		<div class="flex h-full items-center justify-center">
			<p class="text-center text-sm text-ghost">Select a file to view details</p>
		</div>
	{/if}
</div>

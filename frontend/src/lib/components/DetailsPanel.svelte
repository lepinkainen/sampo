<script lang="ts">
import {
	getClassification,
	getDetection,
	getDiskUsage,
	runOCR,
	thumbnailUrl,
} from '$lib/api';
import type {
	ClassificationResult,
	DetectionResult,
	DiskUsage,
	OCRResult,
} from '$lib/api';
import type { FileEntry } from '$lib/types';
import { formatDate, formatSize } from '$lib/utils';
import FileIcon from './FileIcon.svelte';

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

let detailsThumbLoading = $state(false);
let detailsThumbError = $state(false);
let detailsImgEl: HTMLImageElement | undefined = $state();

let lastDetailsThumbKey = $state('');
let lastSelectionDetailsKey = $state('');

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

$effect(() => {
	const thumbKey = sel ? `${rootId}:${sel.path}:${sel.hasThumb}` : '';
	if (thumbKey === lastDetailsThumbKey) {
		return;
	}
	lastDetailsThumbKey = thumbKey;
	detailsThumbError = false;
	detailsThumbLoading = !!sel?.hasThumb;

	// Check if already complete
	if (sel?.hasThumb && detailsImgEl?.complete) {
		detailsThumbLoading = false;
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

<div class="w-80 overflow-y-auto border-l border-gray-800 bg-gray-900 p-6 shadow-xl">
	{#if sel}
		<div class="flex flex-col gap-6">
			<div class="relative aspect-video w-full overflow-hidden rounded-lg bg-gray-950 shadow-inner">
				{#if sel.hasThumb && !detailsThumbError}
					{#if detailsThumbLoading}
						<div
							class="thumb-skeleton absolute inset-0 z-10"
							data-testid="details-thumbnail-skeleton"
						></div>
					{/if}
					<img
						bind:this={detailsImgEl}
						src={thumbnailUrl(rootId, sel.path)}
						alt={sel.name}
						class="relative z-0 h-full w-full object-contain"
						onload={() => (detailsThumbLoading = false)}
						onerror={() => {
							detailsThumbLoading = false;
							detailsThumbError = true;
						}}
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
					{#if !sel.isDir}
						<div class="text-gray-500">Size</div>
						<div class="text-gray-300">{formatSize(sel.size)}</div>
					{/if}

					<div class="text-gray-500">Modified</div>
					<div class="text-gray-300">{formatDate(sel.modTime)}</div>

					<div class="text-gray-500">Path</div>
					<div class="break-all text-gray-300">{sel.path}</div>

					{#if sel.isDir && diskUsageLoading}
						<div class="text-gray-500">Usage</div>
						<div class="text-gray-400">Computing...</div>
					{/if}

					{#if sel.isDir && diskUsage}
						<div class="text-gray-500">Total size</div>
						<div class="text-gray-300">{formatSize(diskUsage.totalSize)}</div>

						<div class="text-gray-500">Files</div>
						<div class="text-gray-300">{diskUsage.fileCount}</div>

						<div class="text-gray-500">Subdirs</div>
						<div class="text-gray-300">{diskUsage.dirCount}</div>
					{/if}

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

					{#if sel.sha256}
						<div class="text-gray-500">SHA256</div>
						<div class="text-gray-300">
							<button
								class="font-mono text-xs break-all text-left hover:text-blue-400 transition-colors"
								title="Click to copy full hash"
								onclick={() => { navigator.clipboard.writeText(sel.sha256 ?? ''); onToast('SHA256 copied', 'success'); }}
							>
								{sel.sha256.slice(0, 16)}...
							</button>
						</div>
					{/if}

					{#if sel.crc32}
						<div class="text-gray-500">CRC32</div>
						<div class="text-gray-300 font-mono text-xs">{sel.crc32}</div>
					{/if}

					{#if selectedDetailsTags.length > 0}
						<div class="col-span-2 border-t border-gray-800 pt-2">
							<div class="text-gray-500 mb-1">Tags</div>
							<div class="flex flex-wrap gap-1">
								{#each selectedDetailsTags as tag}
									<span class="rounded bg-purple-600/80 px-1.5 py-0.5 text-xs text-white" title={`${(tag.score * 100).toFixed(0)}%`}>
										{tag.label} <span class="text-purple-300">{(tag.score * 100).toFixed(0)}%</span>
									</span>
								{/each}
							</div>
						</div>
					{/if}

					{#if !sel.isDir && (sel.mediaType === 'image' || sel.mediaType === 'video')}
						<div class="col-span-2 border-t border-gray-800 pt-2">
							<div class="mb-1 flex items-center justify-between">
								<span class="text-gray-500">OCR text</span>
								<button
									class="rounded bg-gray-700 px-2 py-0.5 text-xs text-gray-200 hover:bg-gray-600 disabled:cursor-not-allowed disabled:opacity-50"
									onclick={() => handleRunOCR(sel)}
									disabled={ocrLoading}
									title="Extract text from this image"
								>
									{ocrLoading ? 'Running…' : ocrResult ? 'Re-run OCR' : 'Run OCR'}
								</button>
							</div>
							{#if ocrError}
								<p class="text-xs text-red-400">{ocrError}</p>
							{:else if ocrResult && ocrResult.blocks.length > 0}
								<div class="flex flex-col gap-0.5">
									{#each ocrResult.blocks as block}
										<span class="rounded bg-gray-800 px-1.5 py-0.5 text-xs break-all text-gray-200">{block.text}</span>
									{/each}
								</div>
							{:else if ocrResult?.text}
								<p class="text-xs break-words whitespace-pre-wrap text-gray-200">{ocrResult.text}</p>
							{:else if ocrResult}
								<p class="text-xs text-gray-500">No text found</p>
							{:else}
								<p class="text-xs text-gray-600">Not analyzed yet</p>
							{/if}
						</div>
					{/if}
				</div>
			</div>

			<div class="mt-auto pt-6">
				<button
					class="w-full rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 focus:ring-offset-gray-900"
					onclick={() => onOpen(sel)}
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
				<div class="text-gray-300">{formatSize(selectedEntries.reduce((sum, e) => (e.isDir ? sum : sum + e.size), 0))}</div>

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

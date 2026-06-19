<script lang="ts">
import type { AnalysisSettings, ScanStatus } from '$lib/api';
import {
	ClipboardPaste,
	Copy,
	Files,
	LayoutGrid,
	List,
	LoaderCircle,
	Pencil,
	RefreshCw,
	ScanSearch,
	ScanText,
	Scissors,
	Search,
	Sparkles,
	Tag,
	Trash2,
	UserX,
	X,
} from '@lucide/svelte';

interface Props {
	rootId: string;
	rootName?: string;
	pathSegments: string[];
	backgroundValidating: boolean;
	selectionSize: number;
	hasClipboard: boolean;
	searchActive: boolean;
	searchQuery: string;
	searchLoading: boolean;
	searchResultCount: number;
	analysisSettings: AnalysisSettings | null;
	analysisSettingsSaving: boolean;
	scanStatus: ScanStatus | null;
	classifyScanStatus: ScanStatus | null;
	ocrScanStatus: ScanStatus | null;
	analyzeScanStatus: ScanStatus | null;
	filterPeople: boolean;
	filterTag: string;
	availableTags: string[];
	viewMode: 'grid' | 'list';
	thumbSize: 'small' | 'medium' | 'large';
	onNavigate: (path: string) => void;
	onOpenSearch: () => void;
	onCloseSearch: () => void;
	onSearchInput: (e: Event) => void;
	onCut: () => void;
	onCopy: () => void;
	onPaste: () => void;
	onRename: () => void;
	onDelete: () => void;
	onToggleAutoBrowse: () => void;
	onToggleFilter: () => void;
	onScan: () => void;
	onClassify: () => void;
	onOCR: () => void;
	onReanalyze: () => void;
	onFindDuplicates: () => void;
	onTagFilter: (e: Event) => void;
	onViewMode: (mode: 'grid' | 'list') => void;
	onThumbSize: (size: 'small' | 'medium' | 'large') => void;
	searchInput?: HTMLInputElement;
}

let {
	rootId,
	rootName,
	pathSegments,
	backgroundValidating,
	selectionSize,
	hasClipboard,
	searchActive,
	searchQuery,
	searchLoading,
	searchResultCount,
	analysisSettings,
	analysisSettingsSaving,
	scanStatus,
	classifyScanStatus,
	ocrScanStatus,
	analyzeScanStatus,
	filterPeople,
	filterTag,
	availableTags,
	viewMode,
	thumbSize,
	onNavigate,
	onOpenSearch,
	onCloseSearch,
	onSearchInput,
	onCut,
	onCopy,
	onPaste,
	onRename,
	onDelete,
	onToggleAutoBrowse,
	onToggleFilter,
	onScan,
	onClassify,
	onOCR,
	onReanalyze,
	onFindDuplicates,
	onTagFilter,
	onViewMode,
	onThumbSize,
	searchInput = $bindable(),
}: Props = $props();
</script>

<div class="flex items-center justify-between border-b border-gray-800 bg-gray-900 px-4 py-2">
	<div class="flex items-center gap-4 min-w-0 flex-1">
		{#if searchActive}
			<div class="flex items-center gap-2 flex-1 max-w-md">
				<Search size={16} class="text-gray-500 shrink-0" />
				<input
					bind:this={searchInput}
					type="text"
					placeholder="Search files and tags..."
					class="flex-1 bg-transparent border-none text-sm text-gray-200 placeholder-gray-500 focus:outline-none"
					value={searchQuery}
					oninput={onSearchInput}
				/>
				{#if searchLoading}
					<span class="text-xs text-gray-500">...</span>
				{/if}
				<button
					class="rounded p-1 text-gray-500 hover:text-gray-300 transition-colors"
					onclick={onCloseSearch}
				>
					<X size={14} />
				</button>
			</div>
		{:else}
			<div class="truncate text-sm font-medium text-gray-300">
				<button
					class="text-gray-500 hover:text-gray-200 transition-colors"
					onclick={() => onNavigate('')}
				>
					{rootName || rootId}
				</button>
				{#each pathSegments as segment, i}
					<span class="mx-1 text-gray-600">/</span>
					{#if i < pathSegments.length - 1}
						<button
							class="text-gray-400 hover:text-gray-200 transition-colors"
							onclick={() => onNavigate(pathSegments.slice(0, i + 1).join('/'))}
						>
							{segment}
						</button>
					{:else}
						<span>{segment}</span>
					{/if}
				{/each}
				{#if backgroundValidating}
					<LoaderCircle size={14} class="animate-spin text-gray-500 ml-2 inline-block align-middle" />
				{/if}
			</div>
		{/if}

		<!-- File operation buttons -->
		<div class="flex items-center gap-1">
			<button
				class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300"
				title="Search (Ctrl+F)"
				onclick={onOpenSearch}
			>
				<Search size={16} />
			</button>

			<div class="mx-1 h-4 w-px bg-gray-700"></div>

			<button
				class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
				title="Cut (Ctrl+X)"
				disabled={selectionSize === 0}
				onclick={onCut}
			>
				<Scissors size={16} />
			</button>
			<button
				class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
				title="Copy (Ctrl+C)"
				disabled={selectionSize === 0}
				onclick={onCopy}
			>
				<Copy size={16} />
			</button>
			<button
				class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
				title="Paste (Ctrl+V)"
				disabled={!hasClipboard}
				onclick={onPaste}
			>
				<ClipboardPaste size={16} />
			</button>
			<button
				class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
				title="Rename (F2)"
				disabled={selectionSize !== 1}
				onclick={onRename}
			>
				<Pencil size={16} />
			</button>
			<button
				class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-red-400 disabled:opacity-30 disabled:cursor-not-allowed"
				title="Delete"
				disabled={selectionSize === 0}
				onclick={onDelete}
			>
				<Trash2 size={16} />
			</button>

			<div class="mx-1 h-4 w-px bg-gray-700"></div>

			<button
				class="rounded px-2 py-1 text-xs font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed {analysisSettings?.autoBrowseEnabled ? 'bg-emerald-600 text-white' : 'text-gray-400 hover:bg-gray-800 hover:text-gray-200'}"
				title="Automatically analyze files while browsing"
				disabled={!analysisSettings || analysisSettingsSaving}
				onclick={onToggleAutoBrowse}
			>
				Auto ML
			</button>
			{#if analysisSettings?.browseStatus.running}
				<div
					class="flex items-center gap-1 rounded bg-amber-500/15 px-2 py-1 text-xs text-amber-300"
					title={`Background analysis running (${analysisSettings.browseStatus.active} active, ${analysisSettings.browseStatus.queued} queued)`}
				>
					<LoaderCircle size={12} class="animate-spin" />
					<span>{analysisSettings.browseStatus.active} active</span>
					{#if analysisSettings.browseStatus.queued > 0}
						<span class="text-amber-400/80">/ {analysisSettings.browseStatus.queued} queued</span>
					{/if}
				</div>
			{/if}
			<button
				class="rounded p-1.5 transition-colors {filterPeople ? 'bg-blue-600 text-white' : 'text-gray-500 hover:bg-gray-800 hover:text-gray-300'}"
				title="Hide images with people"
				onclick={onToggleFilter}
			>
				<UserX size={16} />
			</button>
			<button
				class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
				title="Scan for people"
				disabled={scanStatus?.running === true}
				onclick={onScan}
			>
				<ScanSearch size={16} />
			</button>

			<div class="mx-1 h-4 w-px bg-gray-700"></div>

			<button
				class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
				title="Classify images (CLIP)"
				disabled={classifyScanStatus?.running === true}
				onclick={onClassify}
			>
				<Sparkles size={16} />
			</button>
			<button
				class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
				title="Run OCR on this folder (extract text from images)"
				disabled={ocrScanStatus?.running === true}
				onclick={onOCR}
			>
				<ScanText size={16} />
			</button>
			<button
				class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
				title="Re-analyze this folder and subfolders from scratch — runs detection, tagging, and OCR in one pass (replaces all results)"
				disabled={analyzeScanStatus?.running === true}
				onclick={onReanalyze}
			>
				<RefreshCw size={16} />
			</button>
			<button
				class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300"
				title="Find duplicates"
				onclick={onFindDuplicates}
			>
				<Files size={16} />
			</button>
			{#if availableTags.length > 0}
				<div class="relative flex items-center">
					<Tag size={14} class="absolute left-1.5 text-gray-500 pointer-events-none" />
					<select
						class="appearance-none rounded bg-gray-800 py-1 pl-6 pr-6 text-xs text-gray-300 border border-gray-700 focus:border-blue-500 focus:outline-none"
						value={filterTag}
						onchange={onTagFilter}
					>
						<option value="">All tags</option>
						{#each availableTags as tag}
							<option value={tag}>{tag}</option>
						{/each}
					</select>
				</div>
			{/if}
		</div>
	</div>

	<div class="flex items-center gap-2">
		{#if searchActive && searchQuery && !searchLoading}
			<span class="text-xs text-gray-500">{searchResultCount} result(s)</span>
		{/if}
		{#if scanStatus?.running}
			<span class="text-xs text-blue-400">
				Scanning {scanStatus.completed}/{scanStatus.total}
			</span>
		{/if}
		{#if classifyScanStatus?.running}
			<span class="text-xs text-purple-400">
				Classifying {classifyScanStatus.completed}/{classifyScanStatus.total}
			</span>
		{/if}
		{#if ocrScanStatus?.running}
			<span class="text-xs text-amber-400">
				OCR {ocrScanStatus.completed}/{ocrScanStatus.total}
			</span>
		{/if}
		{#if analyzeScanStatus?.running}
			<span class="text-xs text-emerald-400">
				Analyzing {analyzeScanStatus.completed}/{analyzeScanStatus.total}
			</span>
		{/if}
		{#if selectionSize > 0}
			<span class="text-xs text-gray-500">{selectionSize} selected</span>
		{/if}

		<!-- View mode toggle -->
		<div class="flex items-center gap-1 rounded-lg bg-gray-800 p-1">
			<button
				class="rounded p-1 transition-colors {viewMode === 'grid' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-200'}"
				title="Grid view"
				onclick={() => onViewMode('grid')}
			>
				<LayoutGrid size={14} />
			</button>
			<button
				class="rounded p-1 transition-colors {viewMode === 'list' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-200'}"
				title="List view"
				onclick={() => onViewMode('list')}
			>
				<List size={14} />
			</button>
		</div>

		{#if viewMode === 'grid'}
			<div class="flex items-center gap-1 rounded-lg bg-gray-800 p-1">
				<button
					class="rounded px-2 py-1 text-xs font-medium transition-colors {thumbSize === 'small' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-200'}"
					onclick={() => onThumbSize('small')}
				>
					S
				</button>
				<button
					class="rounded px-2 py-1 text-xs font-medium transition-colors {thumbSize === 'medium' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-200'}"
					onclick={() => onThumbSize('medium')}
				>
					M
				</button>
				<button
					class="rounded px-2 py-1 text-xs font-medium transition-colors {thumbSize === 'large' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-200'}"
					onclick={() => onThumbSize('large')}
				>
					L
				</button>
			</div>
		{/if}
	</div>
</div>

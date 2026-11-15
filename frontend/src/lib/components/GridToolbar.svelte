<script lang="ts">
import type { AnalysisSettings, ScanStatus } from '$lib/api';
import {
	ClipboardPaste,
	Copy,
	Files,
	FolderInput,
	LayoutGrid,
	List,
	Palette,
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
import { cycleTheme, theme } from '$lib/theme.svelte';
import Loader from './Loader.svelte';

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
	onSuggestOrganize?: () => void;
	organizeEnabled?: boolean;
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
	onSuggestOrganize,
	organizeEnabled = false,
	onTagFilter,
	onViewMode,
	onThumbSize,
	searchInput = $bindable(),
}: Props = $props();
</script>

<div class="flex items-center justify-between border-b border-raised bg-surface px-4 py-2">
	<div class="flex items-center gap-4 min-w-0 flex-1">
		{#if searchActive}
			<search
				class="flex items-center gap-2 flex-1 max-w-md rounded has-[input:focus-visible]:ring-1 has-[input:focus-visible]:ring-accent-hover"
			>
				<Search size={16} class="text-faint shrink-0" aria-hidden="true" />
				<input
					bind:this={searchInput}
					type="search"
					aria-label="Search files and tags"
					placeholder="Search files and tags..."
					class="flex-1 bg-transparent border-none text-sm text-body placeholder-faint focus:outline-none [&::-webkit-search-cancel-button]:appearance-none"
					value={searchQuery}
					oninput={onSearchInput}
				/>
				{#if searchLoading}
					<span class="text-xs text-faint">...</span>
				{/if}
				<button
					type="button"
					class="rounded p-1 text-faint hover:text-body transition-colors"
					aria-label="Close search"
					onclick={onCloseSearch}
				>
					<X size={14} aria-hidden="true" />
				</button>
			</search>
		{:else}
			<nav aria-label="Breadcrumbs" class="folder-title truncate text-sm font-medium text-body">
				<button
					type="button"
					class="text-faint hover:text-body transition-colors"
					aria-current={pathSegments.length === 0 ? 'page' : undefined}
					onclick={() => onNavigate('')}
				>
					{rootName || rootId}
				</button>
				{#each pathSegments as segment, i}
					<span class="mx-1 text-ghost" aria-hidden="true">/</span>
					{#if i < pathSegments.length - 1}
						<button
							type="button"
							class="text-dim hover:text-body transition-colors"
							onclick={() => onNavigate(pathSegments.slice(0, i + 1).join('/'))}
						>
							{segment}
						</button>
					{:else}
						<span aria-current="page">{segment}</span>
					{/if}
				{/each}
				{#if backgroundValidating}
					<Loader size={14} class="text-faint ml-2 inline-block align-middle" />
				{/if}
			</nav>
		{/if}

		<!-- File operation buttons -->
		<div class="flex items-center gap-1" role="toolbar" aria-label="File actions">
			<button
				type="button"
				class="rounded p-1.5 text-faint transition-colors hover:bg-raised hover:text-body"
				title="Search (Ctrl+F)"
				aria-label="Search"
				onclick={onOpenSearch}
			>
				<Search size={16} aria-hidden="true" />
			</button>

			<div class="mx-1 h-4 w-px bg-muted"></div>

			<button
				type="button"
				class="rounded p-1.5 text-faint transition-colors hover:bg-raised hover:text-body disabled:opacity-30 disabled:cursor-not-allowed"
				title="Cut (Ctrl+X)"
				aria-label="Cut"
				disabled={selectionSize === 0}
				onclick={onCut}
			>
				<Scissors size={16} aria-hidden="true" />
			</button>
			<button
				type="button"
				class="rounded p-1.5 text-faint transition-colors hover:bg-raised hover:text-body disabled:opacity-30 disabled:cursor-not-allowed"
				title="Copy (Ctrl+C)"
				aria-label="Copy"
				disabled={selectionSize === 0}
				onclick={onCopy}
			>
				<Copy size={16} aria-hidden="true" />
			</button>
			<button
				type="button"
				class="rounded p-1.5 text-faint transition-colors hover:bg-raised hover:text-body disabled:opacity-30 disabled:cursor-not-allowed"
				title="Paste (Ctrl+V)"
				aria-label="Paste"
				disabled={!hasClipboard}
				onclick={onPaste}
			>
				<ClipboardPaste size={16} aria-hidden="true" />
			</button>
			<button
				type="button"
				class="rounded p-1.5 text-faint transition-colors hover:bg-raised hover:text-body disabled:opacity-30 disabled:cursor-not-allowed"
				title="Rename (F2)"
				aria-label="Rename"
				disabled={selectionSize !== 1}
				onclick={onRename}
			>
				<Pencil size={16} aria-hidden="true" />
			</button>
			<button
				type="button"
				class="rounded p-1.5 text-faint transition-colors hover:bg-raised hover:text-danger-soft disabled:opacity-30 disabled:cursor-not-allowed"
				title="Delete"
				aria-label="Delete"
				disabled={selectionSize === 0}
				onclick={onDelete}
			>
				<Trash2 size={16} aria-hidden="true" />
			</button>

			<div class="mx-1 h-4 w-px bg-muted"></div>

			<button
				type="button"
				class="rounded px-2 py-1 text-xs font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed {analysisSettings?.autoBrowseEnabled ? 'bg-ok-strong text-white' : 'text-dim hover:bg-raised hover:text-body'}"
				title="Automatically analyze files while browsing"
				aria-pressed={analysisSettings?.autoBrowseEnabled ?? false}
				disabled={!analysisSettings || analysisSettingsSaving}
				onclick={onToggleAutoBrowse}
			>
				Auto ML
			</button>
			{#if analysisSettings?.browseStatus.running}
				<div
					class="flex items-center gap-1 rounded bg-warn/15 px-2 py-1 text-xs text-warn-soft"
					title={`Background analysis running (${analysisSettings.browseStatus.active} active, ${analysisSettings.browseStatus.queued} queued)`}
				>
				<Loader size={12} speed="1.2s" />
					<span>{analysisSettings.browseStatus.active} active</span>
					{#if analysisSettings.browseStatus.queued > 0}
						<span class="text-warn/80">/ {analysisSettings.browseStatus.queued} queued</span>
					{/if}
				</div>
			{/if}
			<button
				type="button"
				class="rounded p-1.5 transition-colors {filterPeople ? 'bg-accent text-on-accent' : 'text-faint hover:bg-raised hover:text-body'}"
				title="Hide images with people"
				aria-label="Hide images with people"
				aria-pressed={filterPeople}
				onclick={onToggleFilter}
			>
				<UserX size={16} aria-hidden="true" />
			</button>
			<button
				type="button"
				class="rounded p-1.5 text-faint transition-colors hover:bg-raised hover:text-body disabled:opacity-30 disabled:cursor-not-allowed"
				title="Scan for people"
				aria-label="Scan for people"
				disabled={scanStatus?.running === true}
				onclick={onScan}
			>
				<ScanSearch size={16} aria-hidden="true" />
			</button>

			<div class="mx-1 h-4 w-px bg-muted"></div>

			<button
				type="button"
				class="rounded p-1.5 text-faint transition-colors hover:bg-raised hover:text-body disabled:opacity-30 disabled:cursor-not-allowed"
				title="Classify images (CLIP)"
				aria-label="Classify images"
				disabled={classifyScanStatus?.running === true}
				onclick={onClassify}
			>
				<Sparkles size={16} aria-hidden="true" />
			</button>
			<button
				type="button"
				class="rounded p-1.5 text-faint transition-colors hover:bg-raised hover:text-body disabled:opacity-30 disabled:cursor-not-allowed"
				title="Run OCR on this folder (extract text from images)"
				aria-label="Run OCR"
				disabled={ocrScanStatus?.running === true}
				onclick={onOCR}
			>
				<ScanText size={16} aria-hidden="true" />
			</button>
			<button
				type="button"
				class="rounded p-1.5 text-faint transition-colors hover:bg-raised hover:text-body disabled:opacity-30 disabled:cursor-not-allowed"
				title="Re-analyze this folder and subfolders from scratch — runs detection, tagging, and OCR in one pass (replaces all results)"
				aria-label="Re-analyze folder"
				disabled={analyzeScanStatus?.running === true}
				onclick={onReanalyze}
			>
				<RefreshCw size={16} aria-hidden="true" />
			</button>
			<button
				type="button"
				class="rounded p-1.5 text-faint transition-colors hover:bg-raised hover:text-body"
				title="Find duplicates"
				aria-label="Find duplicates"
				onclick={onFindDuplicates}
			>
				<Files size={16} aria-hidden="true" />
			</button>
			{#if organizeEnabled}
				<button
					type="button"
					class="rounded p-1.5 text-faint transition-colors hover:bg-raised hover:text-accent-soft"
					title="Suggest performer folders from Stash"
					aria-label="Suggest performer folders"
					onclick={onSuggestOrganize}
				>
					<FolderInput size={16} aria-hidden="true" />
				</button>
			{/if}
			{#if availableTags.length > 0}
				<div class="relative flex items-center">
					<Tag size={14} class="absolute left-1.5 text-faint pointer-events-none" aria-hidden="true" />
					<select
						aria-label="Filter by tag"
						class="appearance-none rounded bg-raised py-1 pl-6 pr-6 text-xs text-body border border-muted focus:border-accent-hover focus:outline-none"
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
			<span class="text-xs text-faint">{searchResultCount} result(s)</span>
		{/if}
		{#if scanStatus?.running}
			<span class="text-xs text-accent-soft">
				Scanning {scanStatus.completed}/{scanStatus.total}
			</span>
		{/if}
		{#if classifyScanStatus?.running}
			<span class="text-xs text-tag-soft">
				Classifying {classifyScanStatus.completed}/{classifyScanStatus.total}
			</span>
		{/if}
		{#if ocrScanStatus?.running}
			<span class="text-xs text-warn">
				OCR {ocrScanStatus.completed}/{ocrScanStatus.total}
			</span>
		{/if}
		{#if analyzeScanStatus?.running}
			<span class="text-xs text-ok">
				Analyzing {analyzeScanStatus.completed}/{analyzeScanStatus.total}
			</span>
		{/if}
		{#if selectionSize > 0}
			<span class="text-xs text-faint">{selectionSize} selected</span>
		{/if}

		<!-- View mode toggle -->
		<div class="flex items-center gap-1 rounded-lg bg-raised p-1">
			<button
				type="button"
				class="rounded p-1 transition-colors {viewMode === 'grid' ? 'bg-select text-select-text' : 'text-dim hover:text-body'}"
				title="Grid view"
				aria-label="Grid view"
				aria-pressed={viewMode === 'grid'}
				onclick={() => onViewMode('grid')}
			>
				<LayoutGrid size={14} aria-hidden="true" />
			</button>
			<button
				type="button"
				class="rounded p-1 transition-colors {viewMode === 'list' ? 'bg-select text-select-text' : 'text-dim hover:text-body'}"
				title="List view"
				aria-label="List view"
				aria-pressed={viewMode === 'list'}
				onclick={() => onViewMode('list')}
			>
				<List size={14} aria-hidden="true" />
			</button>
		</div>

		<div
			class="flex items-center gap-1 rounded-lg bg-raised p-1 transition-opacity {viewMode === 'grid' ? '' : 'opacity-40'}"
			title={viewMode === 'grid' ? undefined : 'Thumbnail size (grid view only)'}
		>
			<button
				type="button"
				class="rounded px-2 py-1 text-xs font-medium transition-colors {thumbSize === 'small' ? 'bg-select text-select-text' : 'text-dim hover:text-body'} disabled:cursor-not-allowed disabled:hover:text-dim"
				aria-label="Small thumbnails"
				aria-pressed={thumbSize === 'small'}
				disabled={viewMode !== 'grid'}
				onclick={() => onThumbSize('small')}
			>
				S
			</button>
			<button
				type="button"
				class="rounded px-2 py-1 text-xs font-medium transition-colors {thumbSize === 'medium' ? 'bg-select text-select-text' : 'text-dim hover:text-body'} disabled:cursor-not-allowed disabled:hover:text-dim"
				aria-label="Medium thumbnails"
				aria-pressed={thumbSize === 'medium'}
				disabled={viewMode !== 'grid'}
				onclick={() => onThumbSize('medium')}
			>
				M
			</button>
			<button
				type="button"
				class="rounded px-2 py-1 text-xs font-medium transition-colors {thumbSize === 'large' ? 'bg-select text-select-text' : 'text-dim hover:text-body'} disabled:cursor-not-allowed disabled:hover:text-dim"
				aria-label="Large thumbnails"
				aria-pressed={thumbSize === 'large'}
				disabled={viewMode !== 'grid'}
				onclick={() => onThumbSize('large')}
			>
				L
			</button>
		</div>

		<button
			type="button"
			class="rounded p-1.5 text-faint transition-colors hover:bg-raised hover:text-body"
			title="Theme: {theme.current} (click to switch)"
			aria-label="Switch theme"
			onclick={cycleTheme}
		>
			<Palette size={16} aria-hidden="true" />
		</button>
	</div>
</div>


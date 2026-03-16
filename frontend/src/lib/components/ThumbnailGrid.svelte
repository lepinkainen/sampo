<script lang="ts">
	import type { FileEntry } from '$lib/types';
	import { fetchDirectory, thumbnailUrl } from '$lib/api';
	import ThumbnailCard from './ThumbnailCard.svelte';
	import MediaPreview from './MediaPreview.svelte';

	interface Props {
		rootId: string;
		path: string;
		onNavigate?: (path: string) => void;
	}

	let { rootId, path, onNavigate }: Props = $props();

	let entries = $state<FileEntry[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let previewIndex = $state<number | null>(null);
	let thumbSize = $state<'small' | 'medium' | 'large'>('medium');
	let selectedEntry = $state<FileEntry | null>(null);

	let mediaEntries = $derived(entries.filter((e) => e.mediaType === 'image' || e.mediaType === 'video'));

	let gridClass = $derived.by(() => {
		switch (thumbSize) {
			case 'small': return 'grid-cols-[repeat(auto-fill,minmax(120px,1fr))]';
			case 'medium': return 'grid-cols-[repeat(auto-fill,minmax(180px,1fr))]';
			case 'large': return 'grid-cols-[repeat(auto-fill,minmax(280px,1fr))]';
		}
	});

	$effect(() => {
		loadDirectory(rootId, path);
	});

	async function loadDirectory(rid: string, p: string) {
		previewIndex = null;
		selectedEntry = null;
		loading = true;
		error = null;
		try {
			const result = await fetchDirectory(rid, p);
			// Sort: directories first, then by name
			result.sort((a, b) => {
				if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
				return a.name.localeCompare(b.name);
			});
			entries = result;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load directory';
			entries = [];
		}
		loading = false;
	}

	function openPreview(entry: FileEntry) {
		const idx = mediaEntries.findIndex((e) => e.path === entry.path);
		if (idx >= 0) previewIndex = idx;
	}

	function handleSelect(entry: FileEntry) {
		selectedEntry = entry;
	}

	function handleOpen(entry: FileEntry) {
		if (entry.isDir) {
			onNavigate?.(entry.path);
		} else if (entry.mediaType === 'image' || entry.mediaType === 'video') {
			openPreview(entry);
		}
	}

	function formatSize(bytes: number): string {
		if (bytes < 1024) return `${bytes} B`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
		return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
	}

	function formatDate(dateStr: string): string {
		try {
			return new Date(dateStr).toLocaleString();
		} catch {
			return dateStr;
		}
	}
</script>

{#if previewIndex !== null}
	<MediaPreview
		{rootId}
		{mediaEntries}
		initialIndex={previewIndex}
		onClose={() => (previewIndex = null)}
	/>
{:else}
	<div class="flex h-full flex-col bg-gray-950">
		<!-- Toolbar -->
		<div class="flex items-center justify-between border-b border-gray-800 bg-gray-900 px-4 py-2">
			<div class="truncate text-sm font-medium text-gray-300">
				<span class="text-gray-500">{rootId}</span>
				<span class="mx-1 text-gray-600">/</span>
				{path || '(root)'}
			</div>
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

		<div class="flex flex-1 overflow-hidden">
			<div
				class="flex-1 overflow-y-auto p-4"
				onclick={() => (selectedEntry = null)}
				onkeydown={() => {}}
				role="button"
				tabindex="-1"
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
								selected={selectedEntry?.path === entry.path}
								onclick={() => handleSelect(entry)}
								ondblclick={() => handleOpen(entry)}
							/>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Details Panel -->
			{#if selectedEntry}
				<div class="w-80 overflow-y-auto border-l border-gray-800 bg-gray-900 p-6 shadow-xl">
					<div class="flex flex-col gap-6">
						<div class="aspect-video w-full overflow-hidden rounded-lg bg-gray-950 shadow-inner">
							{#if selectedEntry.hasThumb}
								<img
									src={thumbnailUrl(rootId, selectedEntry.path)}
									alt={selectedEntry.name}
									class="h-full w-full object-contain"
								/>
							{:else}
								<div class="flex h-full items-center justify-center text-6xl text-gray-700">
									{#if selectedEntry.isDir}
										&#128193;
									{:else if selectedEntry.mediaType === 'image'}
										&#128444;
									{:else if selectedEntry.mediaType === 'video'}
										&#127909;
									{:else}
										&#128196;
									{/if}
								</div>
							{/if}
						</div>

						<div class="space-y-4">
							<div>
								<h3 class="break-all text-lg font-semibold text-gray-100">{selectedEntry.name}</h3>
								<p class="text-sm text-gray-400">{selectedEntry.mediaType}</p>
							</div>

							<div class="grid grid-cols-2 gap-y-4 text-sm">
								<div class="text-gray-500">Size</div>
								<div class="text-gray-300">{formatSize(selectedEntry.size)}</div>

								<div class="text-gray-500">Modified</div>
								<div class="text-gray-300">{formatDate(selectedEntry.modTime)}</div>

								<div class="text-gray-500">Path</div>
								<div class="break-all text-gray-300">{selectedEntry.path}</div>
							</div>
						</div>

						<div class="mt-auto pt-6">
							<button
								class="w-full rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 focus:ring-offset-gray-900"
								onclick={() => handleOpen(selectedEntry!)}
							>
								{selectedEntry.isDir ? 'Open Folder' : 'Open'}
							</button>
						</div>
					</div>
				</div>
			{/if}
		</div>
	</div>
{/if}

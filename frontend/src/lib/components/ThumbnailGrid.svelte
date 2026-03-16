<script lang="ts">
	import type { FileEntry } from '$lib/types';
	import { fetchDirectory } from '$lib/api';
	import ThumbnailCard from './ThumbnailCard.svelte';
	import MediaPreview from './MediaPreview.svelte';

	interface Props {
		rootId: string;
		path: string;
	}

	let { rootId, path }: Props = $props();

	let entries = $state<FileEntry[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let previewIndex = $state<number | null>(null);

	let mediaEntries = $derived(entries.filter((e) => e.mediaType === 'image' || e.mediaType === 'video'));

	$effect(() => {
		loadDirectory(rootId, path);
	});

	async function loadDirectory(rid: string, p: string) {
		previewIndex = null;
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
</script>

{#if previewIndex !== null}
	<MediaPreview
		{rootId}
		{mediaEntries}
		initialIndex={previewIndex}
		onClose={() => (previewIndex = null)}
	/>
{:else}
	<div class="h-full overflow-y-auto bg-gray-950 p-4">
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
			<div class="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-3">
				{#each entries as entry (entry.path)}
					<ThumbnailCard
						{rootId}
						{entry}
						onclick={entry.mediaType === 'image' || entry.mediaType === 'video'
							? () => openPreview(entry)
							: undefined}
					/>
				{/each}
			</div>
		{/if}
	</div>
{/if}

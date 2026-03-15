<script lang="ts">
	import type { FileEntry } from '$lib/types';
	import { thumbnailUrl } from '$lib/api';

	interface Props {
		rootId: string;
		entry: FileEntry;
	}

	let { rootId, entry }: Props = $props();

	let imgError = $state(false);

	function formatSize(bytes: number): string {
		if (bytes < 1024) return `${bytes} B`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
		return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
	}
</script>

<div
	class="group flex flex-col overflow-hidden rounded-lg border border-gray-700 bg-gray-800 transition-colors hover:border-gray-500"
>
	<div class="flex aspect-square items-center justify-center bg-gray-900">
		{#if entry.hasThumb && !imgError}
			<img
				src={thumbnailUrl(rootId, entry.path)}
				alt={entry.name}
				class="h-full w-full object-cover"
				loading="lazy"
				onerror={() => (imgError = true)}
			/>
		{:else}
			<span class="text-4xl">
				{#if entry.isDir}
					&#128193;
				{:else if entry.mediaType === 'image'}
					&#128444;
				{:else if entry.mediaType === 'video'}
					&#127909;
				{:else if entry.mediaType === 'archive'}
					&#128230;
				{:else}
					&#128196;
				{/if}
			</span>
		{/if}
	</div>
	<div class="p-2">
		<p class="truncate text-sm text-gray-200" title={entry.name}>{entry.name}</p>
		<p class="text-xs text-gray-500">{formatSize(entry.size)}</p>
	</div>
</div>

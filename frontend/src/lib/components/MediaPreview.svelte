<script lang="ts">
	import type { FileEntry } from '$lib/types';
	import { fileUrl } from '$lib/api';

	interface Props {
		rootId: string;
		mediaEntries: FileEntry[];
		initialIndex: number;
		onClose: () => void;
	}

	let { rootId, mediaEntries, initialIndex, onClose }: Props = $props();

	// svelte-ignore state_referenced_locally
	let currentIndex = $state(initialIndex);
	let currentEntry = $derived(mediaEntries[currentIndex]);

	function prev() {
		if (currentIndex > 0) currentIndex--;
	}

	function next() {
		if (currentIndex < mediaEntries.length - 1) currentIndex++;
	}

	$effect(() => {
		function onKeyDown(e: KeyboardEvent) {
			if (e.key === 'ArrowLeft') {
				e.preventDefault();
				prev();
			} else if (e.key === 'ArrowRight') {
				e.preventDefault();
				next();
			} else if (e.key === 'Escape') {
				e.preventDefault();
				onClose();
			}
		}
		window.addEventListener('keydown', onKeyDown);
		return () => window.removeEventListener('keydown', onKeyDown);
	});
</script>

<div class="flex h-full flex-col bg-gray-950">
	<!-- Top bar -->
	<div class="flex items-center justify-between border-b border-gray-800 px-4 py-2">
		<div class="min-w-0 flex-1">
			<p class="truncate text-sm text-gray-200" title={currentEntry.name}>{currentEntry.name}</p>
		</div>
		<div class="flex items-center gap-3">
			<span class="text-xs text-gray-500">{currentIndex + 1} / {mediaEntries.length}</span>
			<button
				onclick={onClose}
				class="rounded p-1 text-gray-400 hover:bg-gray-800 hover:text-gray-200"
				aria-label="Close preview"
			>
				&#10005;
			</button>
		</div>
	</div>

	<!-- Media area -->
	<div class="relative flex flex-1 items-center justify-center overflow-hidden">
		<!-- Prev button -->
		{#if currentIndex > 0}
			<button
				onclick={prev}
				class="absolute left-2 z-10 rounded-full bg-gray-900/70 p-2 text-gray-300 hover:bg-gray-800 hover:text-white"
				aria-label="Previous"
			>
				&#9664;
			</button>
		{/if}

		{#if currentEntry.mediaType === 'video'}
			{#key currentEntry.path}
				<!-- svelte-ignore a11y_media_has_caption -->
				<video
					src={fileUrl(rootId, currentEntry.path)}
					controls
					class="max-h-full max-w-full"
				></video>
			{/key}
		{:else}
			<img
				src={fileUrl(rootId, currentEntry.path)}
				alt={currentEntry.name}
				class="max-h-full max-w-full object-contain"
			/>
		{/if}

		<!-- Next button -->
		{#if currentIndex < mediaEntries.length - 1}
			<button
				onclick={next}
				class="absolute right-2 z-10 rounded-full bg-gray-900/70 p-2 text-gray-300 hover:bg-gray-800 hover:text-white"
				aria-label="Next"
			>
				&#9654;
			</button>
		{/if}
	</div>
</div>

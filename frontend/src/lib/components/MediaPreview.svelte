<script lang="ts">
import { fade } from 'svelte/transition';
import { fileUrl } from '$lib/api';
import type { FileEntry } from '$lib/types';
import { X, ChevronLeft, ChevronRight } from '@lucide/svelte';

interface Props {
	rootId: string;
	mediaEntries: FileEntry[];
	currentIndex: number;
	onClose: () => void;
	onIndexChange: (index: number) => void;
}

let { rootId, mediaEntries, currentIndex, onClose, onIndexChange }: Props =
	$props();

let currentEntry = $derived(mediaEntries[currentIndex]);
let wrapNotice = $state<string | null>(null);
let videoLoadError = $state(false);
let wrapTimeout: ReturnType<typeof setTimeout> | null = null;

function showWrapNotice(msg: string) {
	if (wrapTimeout) clearTimeout(wrapTimeout);
	wrapNotice = msg;
	wrapTimeout = setTimeout(() => {
		wrapNotice = null;
	}, 1500);
}

function prev() {
	if (currentIndex > 0) {
		onIndexChange(currentIndex - 1);
	} else {
		onIndexChange(mediaEntries.length - 1);
		showWrapNotice('Wrapped to last');
	}
}

function next() {
	if (currentIndex < mediaEntries.length - 1) {
		onIndexChange(currentIndex + 1);
	} else {
		onIndexChange(0);
		showWrapNotice('Wrapped to first');
	}
}

$effect(() => {
	currentEntry.path;
	videoLoadError = false;
});

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
	return () => {
		window.removeEventListener('keydown', onKeyDown);
		if (wrapTimeout) clearTimeout(wrapTimeout);
	};
});
</script>

<div class="flex h-full flex-col bg-canvas">
	<!-- Top bar -->
	<div class="flex items-center justify-between border-b border-raised px-4 py-2">
		<div class="min-w-0 flex-1">
			<p class="truncate text-sm text-body" title={currentEntry.name}>{currentEntry.name}</p>
		</div>
		<div class="flex items-center gap-3">
			<span class="text-xs text-faint">{currentIndex + 1} / {mediaEntries.length}</span>
			<button
				onclick={onClose}
				class="rounded p-1 text-dim hover:bg-raised hover:text-body"
				aria-label="Close preview"
			>
				<X size={18} />
			</button>
		</div>
	</div>

	<!-- Media area -->
	<div class="relative flex flex-1 items-center justify-center overflow-hidden">
		<!-- Prev button -->
		<button
			onclick={prev}
			class="absolute left-2 z-10 rounded-full bg-surface/70 p-2 text-body hover:bg-raised hover:text-bright"
			aria-label="Previous"
		>
			<ChevronLeft size={24} />
		</button>

		{#if currentEntry.mediaType === 'video'}
			{#key currentEntry.path}
				<div class="flex max-h-full max-w-full flex-col items-center justify-center gap-4 px-6 text-center">
					<!-- svelte-ignore a11y_media_has_caption -->
					<video
						src={fileUrl(rootId, currentEntry.path)}
						controls
						onerror={() => {
							videoLoadError = true;
						}}
						class:max-h-full={!videoLoadError}
						class:max-w-full={!videoLoadError}
						class:hidden={videoLoadError}
					></video>

					{#if videoLoadError}
						<div class="max-w-lg rounded-xl border border-raised bg-surface/80 p-5 text-sm text-body shadow-lg">
							<p class="font-medium text-bright">Preview unavailable in this browser</p>
							<p class="mt-2 text-dim">
								This video file may use an unsupported container or codec.
								MKV files especially are not playable in all browsers.
							</p>
							<p class="mt-3 text-faint">You can still open or download the file directly.</p>
							<a
								href={fileUrl(rootId, currentEntry.path)}
								target="_blank"
								rel="noopener noreferrer"
								class="mt-4 inline-flex rounded bg-bright px-3 py-2 text-sm font-medium text-canvas hover:bg-bright/90"
							>
								Open file
							</a>
						</div>
					{/if}
				</div>
			{/key}
		{:else}
			<img
				src={fileUrl(rootId, currentEntry.path)}
				alt={currentEntry.name}
				class="max-h-full max-w-full object-contain"
			/>
		{/if}

		<!-- Next button -->
		<button
			onclick={next}
			class="absolute right-2 z-10 rounded-full bg-surface/70 p-2 text-body hover:bg-raised hover:text-bright"
			aria-label="Next"
		>
			<ChevronRight size={24} />
		</button>

		<!-- Wrap notification -->
		{#if wrapNotice}
			<div
				class="absolute bottom-6 left-1/2 -translate-x-1/2 rounded-full bg-raised/90 px-4 py-2 text-sm text-body"
				transition:fade={{ duration: 200 }}
			>
				{wrapNotice}
			</div>
		{/if}
	</div>
</div>

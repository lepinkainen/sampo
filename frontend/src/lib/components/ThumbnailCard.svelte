<script lang="ts">
import { onMount } from 'svelte';
import { ThumbnailLoader, thumbnailKey } from '$lib/thumbnailLoader.svelte';
import type { FileEntry } from '$lib/types';
import { formatSize } from '$lib/utils';
import FileIcon from './FileIcon.svelte';
import { Folder, LoaderCircle, ScanText, User } from '@lucide/svelte';

interface Props {
	rootId: string;
	entry: FileEntry;
	selected?: boolean;
	cut?: boolean;
	onclick?: (e: MouseEvent) => void;
	ondblclick?: () => void;
	oncontextmenu?: (e: MouseEvent) => void;
	ondragstart?: (e: DragEvent) => void;
}

let {
	rootId,
	entry,
	selected = false,
	cut = false,
	onclick,
	ondblclick,
	oncontextmenu,
	ondragstart,
}: Props = $props();

const loader = new ThumbnailLoader();

let thumbBox: HTMLDivElement | undefined = $state();
let lastThumbKey = $state('');
let thumbVisible = $state(false);

let thumbKey = $derived(thumbnailKey(rootId, entry));
let showThumbPending = $derived(
	entry.hasThumb && loader.state !== 'ready' && loader.state !== 'error',
);
let showThumbImage = $derived(
	entry.hasThumb && loader.state === 'ready' && loader.objectUrl !== null,
);
let showThumbError = $derived(entry.hasThumb && loader.state === 'error');

onMount(() => {
	if (!thumbBox || typeof IntersectionObserver === 'undefined') {
		thumbVisible = true;
		return () => loader.cleanup();
	}

	const observer = new IntersectionObserver(
		(entries) => {
			if (entries.some((item) => item.isIntersecting)) {
				thumbVisible = true;
				observer.disconnect();
			}
		},
		{ rootMargin: '500px' },
	);
	observer.observe(thumbBox);

	return () => {
		observer.disconnect();
		loader.cleanup();
	};
});

$effect(() => {
	if (thumbKey === lastThumbKey) {
		return;
	}
	lastThumbKey = thumbKey;
	loader.reset(entry.hasThumb ? 'idle' : 'error');
});

$effect(() => {
	if (!thumbVisible || !entry.hasThumb || loader.state !== 'idle') {
		return;
	}
	void loader.fetch(rootId, entry.path);
});
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<div
	data-testid="thumbnail-card"
	class="group flex flex-col overflow-hidden rounded-lg border transition-colors
	{selected
		? 'border-blue-500 bg-blue-900/30'
		: 'border-gray-700 bg-gray-800 hover:border-gray-500'}
	{cut ? ' opacity-50' : ''}
	{onclick ? ' cursor-pointer' : ''}"
	role={onclick ? 'button' : undefined}
	tabindex={onclick ? 0 : undefined}
	draggable={ondragstart ? true : undefined}
	{onclick}
	{ondblclick}
	{oncontextmenu}
	{ondragstart}
	onkeydown={onclick
		? (e) => {
				if (e.key === 'Enter' || e.key === ' ') {
					e.preventDefault();
					onclick(new MouseEvent('click'));
				}
			}
		: undefined}
>
	<div
		bind:this={thumbBox}
		class="relative flex aspect-square items-center justify-center bg-gray-900"
	>
		{#if showThumbImage && loader.objectUrl}
			<img
				src={loader.objectUrl}
				alt={entry.name}
				class="relative z-0 h-full w-full object-cover"
				onerror={() => loader.fail()}
			/>
		{:else if showThumbPending}
			<div
				class="thumb-skeleton absolute inset-0 z-10"
				data-testid="thumbnail-skeleton"
			></div>
			{#if loader.showSlowLoading}
				<div
					class="absolute inset-0 z-20 flex items-center justify-center text-gray-500"
					aria-label="Loading thumbnail"
				>
					<LoaderCircle size={16} class="animate-spin" />
				</div>
			{/if}
		{:else}
			<span class="text-gray-500">
				<FileIcon {entry} size={48} />
			</span>
		{/if}
		{#if entry.tags && entry.tags.length > 0}
			<div class="absolute bottom-1 left-1 flex flex-wrap gap-0.5">
				{#each entry.tags.slice(0, 3) as tag}
					<span class="rounded bg-purple-600/80 px-1 py-0.5 text-[10px] font-medium leading-none text-white">
						{tag.label}
					</span>
				{/each}
			</div>
		{/if}
		<div class="absolute bottom-1 right-1 flex items-center gap-0.5">
			{#if entry.ocrText}
				<span class="rounded bg-black/60 p-0.5 text-white" title="Contains text (OCR)">
					<ScanText size={14} />
				</span>
			{/if}
			{#if entry.isDir && entry.hasThumb && !showThumbError}
				<span class="rounded bg-black/60 p-0.5 text-white">
					<Folder size={14} />
				</span>
			{:else if entry.hasPerson === true}
				<span class="rounded bg-black/60 p-0.5 text-white">
					<User size={14} />
				</span>
			{/if}
		</div>
	</div>
	<div class="p-2">
		<p class="truncate text-sm text-gray-200" title={entry.name}>{entry.name}</p>
		{#if !entry.isDir}
			<p class="text-xs text-gray-500">{formatSize(entry.size)}</p>
		{/if}
	</div>
</div>

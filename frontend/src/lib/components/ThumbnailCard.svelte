<script lang="ts">
import { thumbnailUrl } from '$lib/api';
import type { FileEntry } from '$lib/types';
import { formatSize } from '$lib/utils';
import FileIcon from './FileIcon.svelte';
import { User } from '@lucide/svelte';

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

let imgLoading = $state(entry.hasThumb);
let imgError = $state(false);
let lastThumbKey = $state(`${rootId}:${entry.path}:${entry.hasThumb}`);
let imgEl: HTMLImageElement | undefined = $state();

$effect(() => {
	const thumbKey = `${rootId}:${entry.path}:${entry.hasThumb}`;
	if (thumbKey === lastThumbKey) {
		return;
	}
	lastThumbKey = thumbKey;
	imgLoading = entry.hasThumb;
	imgError = false;

	// Check if already complete (e.g. from cache)
	if (entry.hasThumb && imgEl?.complete) {
		imgLoading = false;
	}
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
	<div class="relative flex aspect-square items-center justify-center bg-gray-900">
		{#if entry.hasThumb && !imgError}
			{#if imgLoading}
				<div
					class="thumb-skeleton absolute inset-0 z-10"
					data-testid="thumbnail-skeleton"
				></div>
			{/if}
			<img
				bind:this={imgEl}
				src={thumbnailUrl(rootId, entry.path)}
				alt={entry.name}
				class="relative z-0 h-full w-full object-cover"
				loading="lazy"
				onload={() => (imgLoading = false)}
				onerror={() => {
					imgLoading = false;
					imgError = true;
				}}
			/>
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
		{#if entry.hasPerson === true}
			<span class="absolute bottom-1 right-1 rounded bg-black/60 p-0.5 text-white">
				<User size={14} />
			</span>
		{/if}
	</div>
	<div class="p-2">
		<p class="truncate text-sm text-gray-200" title={entry.name}>{entry.name}</p>
		<p class="text-xs text-gray-500">{formatSize(entry.size)}</p>
	</div>
</div>

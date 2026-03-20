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

let imgError = $state(false);
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<div
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
			<img
				src={thumbnailUrl(rootId, entry.path)}
				alt={entry.name}
				class="h-full w-full object-cover"
				loading="lazy"
				onerror={() => (imgError = true)}
			/>
		{:else}
			<span class="text-gray-500">
				<FileIcon {entry} size={48} />
			</span>
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

<script lang="ts">
import { fetchDirectory } from '$lib/api';
import type { FileEntry } from '$lib/types';
import { sortEntries } from '$lib/utils';
import { ChevronDown, ChevronRight, Loader } from '@lucide/svelte';
import FileIcon from './FileIcon.svelte';
import TreeNode from './TreeNode.svelte';

interface Props {
	rootId: string;
	entry: FileEntry;
	depth?: number;
	selectedPath: string | null;
	onSelect: (rootId: string, path: string, isDir: boolean) => void;
}

let { rootId, entry, depth = 0, selectedPath, onSelect }: Props = $props();

let expanded = $state(false);
let children = $state<FileEntry[]>([]);
let loading = $state(false);

const isSelected = $derived(selectedPath === `${rootId}:${entry.path}`);

async function toggle() {
	if (!entry.isDir && !entry.isZip) {
		onSelect(rootId, entry.path, false);
		return;
	}

	if (!expanded) {
		loading = true;
		try {
			children = sortEntries(await fetchDirectory(rootId, entry.path));
		} catch (e) {
			console.error('Failed to load directory', e);
		}
		loading = false;
	}

	expanded = !expanded;
	onSelect(rootId, entry.path, true);
}
</script>

<div class="select-none">
	<button
		class="flex min-w-0 w-full items-center gap-1 rounded px-1 py-0.5 text-left text-sm hover:bg-gray-700 {isSelected
			? 'bg-gray-700 text-white'
			: 'text-gray-300'}"
		style="padding-left: {depth * 16 + 4}px"
		onclick={toggle}
	>
		{#if entry.isDir || entry.isZip}
			<span class="w-4 shrink-0 text-gray-500">
				{#if loading}
					<Loader size={14} class="animate-spin" />
				{:else if expanded}
					<ChevronDown size={14} />
				{:else}
					<ChevronRight size={14} />
				{/if}
			</span>
		{:else}
			<span class="w-4 shrink-0"></span>
		{/if}

		<span class="w-5 shrink-0 text-gray-400">
			<FileIcon {entry} size={16} />
		</span>
		<span class="truncate">{entry.name}</span>
	</button>

	{#if expanded && children.length > 0}
		{#each children as child (child.path)}
			<TreeNode
				{rootId}
				entry={child}
				depth={depth + 1}
				{selectedPath}
				{onSelect}
			/>
		{/each}
	{/if}
</div>

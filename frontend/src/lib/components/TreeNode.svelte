<script lang="ts">
	import type { FileEntry } from '$lib/types';
	import { fetchDirectory } from '$lib/api';
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
				children = await fetchDirectory(rootId, entry.path);
				// Sort: directories first, then alphabetical
				children.sort((a, b) => {
					if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
					return a.name.localeCompare(b.name);
				});
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
		class="flex w-full items-center gap-1 rounded px-1 py-0.5 text-left text-sm hover:bg-gray-700 {isSelected
			? 'bg-gray-700 text-white'
			: 'text-gray-300'}"
		style="padding-left: {depth * 16 + 4}px"
		onclick={toggle}
	>
		{#if entry.isDir || entry.isZip}
			<span class="w-4 text-center text-xs text-gray-500">
				{#if loading}
					...
				{:else if expanded}
					&#9662;
				{:else}
					&#9656;
				{/if}
			</span>
		{:else}
			<span class="w-4"></span>
		{/if}

		<span class="truncate">
			{#if entry.isDir}
				&#128193;
			{:else if entry.isZip}
				&#128230;
			{:else if entry.mediaType === 'image'}
				&#128444;
			{:else if entry.mediaType === 'video'}
				&#127909;
			{:else}
				&#128196;
			{/if}
			{entry.name}
		</span>
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

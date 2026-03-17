<script lang="ts">
import { onMount } from 'svelte';
import { fetchDirectory, fetchRoots } from '$lib/api';
import type { FileEntry, Root } from '$lib/types';
import { sortEntries } from '$lib/utils';
import TreeNode from './TreeNode.svelte';

interface Props {
	selectedPath: string | null;
	onSelect: (rootId: string, path: string, isDir: boolean) => void;
}

let { selectedPath, onSelect }: Props = $props();

let roots = $state<Root[]>([]);
let rootChildren = $state<Record<string, FileEntry[]>>({});
let expandedRoots = $state<Set<string>>(new Set());
let loading = $state(true);

onMount(async () => {
	try {
		roots = await fetchRoots();
	} catch (e) {
		console.error('Failed to load roots', e);
	}
	loading = false;
});

async function toggleRoot(rootId: string) {
	if (expandedRoots.has(rootId)) {
		expandedRoots.delete(rootId);
		expandedRoots = new Set(expandedRoots);
	} else {
		if (!rootChildren[rootId]) {
			try {
				const entries = sortEntries(await fetchDirectory(rootId, '/'));
				rootChildren[rootId] = entries;
			} catch (e) {
				console.error('Failed to load root', rootId, e);
			}
		}
		expandedRoots.add(rootId);
		expandedRoots = new Set(expandedRoots);
		onSelect(rootId, '/', true);
	}
}
</script>

<div class="h-full overflow-y-auto bg-gray-900 p-2">
	{#if loading}
		<p class="text-sm text-gray-500">Loading...</p>
	{:else if roots.length === 0}
		<p class="text-sm text-gray-500">No roots configured</p>
	{:else}
		{#each roots as root (root.id)}
			<div class="mb-1">
				<button
					class="flex w-full items-center gap-1 rounded px-1 py-1 text-left text-sm font-semibold text-gray-200 hover:bg-gray-700"
					onclick={() => toggleRoot(root.id)}
				>
					<span class="w-4 text-center text-xs text-gray-500">
						{#if expandedRoots.has(root.id)}
							&#9662;
						{:else}
							&#9656;
						{/if}
					</span>
					&#128193; {root.name}
				</button>

				{#if expandedRoots.has(root.id) && rootChildren[root.id]}
					{#each rootChildren[root.id] as entry (entry.path)}
						<TreeNode
							rootId={root.id}
							{entry}
							depth={1}
							{selectedPath}
							{onSelect}
						/>
					{/each}
				{/if}
			</div>
		{/each}
	{/if}
</div>

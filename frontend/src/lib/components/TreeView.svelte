<script lang="ts">
import { onMount } from 'svelte';
import { fetchDirectory, fetchRoots, moveFiles, copyFiles } from '$lib/api';
import type { FileEntry, Root } from '$lib/types';
import { sortEntries } from '$lib/utils';
import { Folder, ChevronDown, ChevronRight } from '@lucide/svelte';
import TreeNode from './TreeNode.svelte';

interface Props {
	selectedPath: string | null;
	onSelect: (rootId: string, path: string, isDir: boolean) => void;
	onRefresh?: () => void;
}

let { selectedPath, onSelect, onRefresh }: Props = $props();

let roots = $state<Root[]>([]);
let rootChildren = $state<Record<string, FileEntry[]>>({});
let expandedRoots = $state<Set<string>>(new Set());
let loading = $state(true);
let dragOverRoot = $state<string | null>(null);

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

function handleRootDragOver(e: DragEvent, rootId: string) {
	e.preventDefault();
	e.stopPropagation();
	dragOverRoot = rootId;
	if (e.dataTransfer) {
		e.dataTransfer.dropEffect = e.altKey ? 'copy' : 'move';
	}
}

function handleRootDragLeave() {
	dragOverRoot = null;
}

async function handleRootDrop(e: DragEvent, rootId: string) {
	e.preventDefault();
	e.stopPropagation();
	dragOverRoot = null;

	const data = e.dataTransfer?.getData('application/json');
	if (!data) return;

	try {
		const payload = JSON.parse(data) as {
			rootId: string;
			paths: string[];
			mode: 'move' | 'copy';
		};
		const op = payload.mode === 'copy' ? copyFiles : moveFiles;
		await op({
			items: payload.paths.map((p: string) => ({
				srcRoot: payload.rootId,
				srcPath: p,
			})),
			dstRoot: rootId,
			dstPath: '/',
		});
		onRefresh?.();
	} catch {
		// ignore
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
					class="flex min-w-0 w-full items-center gap-1 rounded px-1 py-1 text-left text-sm font-semibold text-gray-200 hover:bg-gray-700
					{dragOverRoot === root.id ? 'ring-2 ring-blue-500 bg-blue-900/20' : ''}"
					onclick={() => toggleRoot(root.id)}
					ondragover={(e) => handleRootDragOver(e, root.id)}
					ondragleave={handleRootDragLeave}
					ondrop={(e) => handleRootDrop(e, root.id)}
				>
					<span class="w-4 shrink-0 text-gray-500">
						{#if expandedRoots.has(root.id)}
							<ChevronDown size={14} />
						{:else}
							<ChevronRight size={14} />
						{/if}
					</span>
					<span class="w-5 shrink-0 text-gray-400"><Folder size={16} /></span>
					<span class="truncate">{root.name}</span>
				</button>

				{#if expandedRoots.has(root.id) && rootChildren[root.id]}
					{#each rootChildren[root.id] as entry (entry.path)}
						<TreeNode
							rootId={root.id}
							{entry}
							depth={1}
							{selectedPath}
							{onSelect}
							{onRefresh}
						/>
					{/each}
				{/if}
			</div>
		{/each}
	{/if}
</div>

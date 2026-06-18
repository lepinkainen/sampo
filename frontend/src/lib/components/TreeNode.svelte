<script lang="ts">
import { fetchDirectory, moveFiles, copyFiles } from '$lib/api';
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
	onRefresh?: () => void;
}

let {
	rootId,
	entry,
	depth = 0,
	selectedPath,
	onSelect,
	onRefresh,
}: Props = $props();

let expanded = $state(false);
let children = $state<FileEntry[]>([]);
let loading = $state(false);
let loadingSlow = $state(false);
let loadingTimer: ReturnType<typeof setTimeout> | null = null;
let dragOver = $state(false);

const isSelected = $derived(selectedPath === `${rootId}:${entry.path}`);

$effect(() => {
	return () => {
		if (loadingTimer) {
			clearTimeout(loadingTimer);
			loadingTimer = null;
		}
	};
});

async function toggle() {
	if (!entry.isDir) {
		onSelect(rootId, entry.path, false);
		return;
	}

	if (!expanded) {
		if (loading) return;
		loading = true;
		loadingSlow = false;
		if (loadingTimer) clearTimeout(loadingTimer);
		loadingTimer = setTimeout(() => {
			loadingSlow = true;
		}, 3000);
		try {
			children = sortEntries(await fetchDirectory(rootId, entry.path)).filter(
				(e) => e.isDir,
			);
		} catch (e) {
			console.error('Failed to load directory', e);
		}
		if (loadingTimer) {
			clearTimeout(loadingTimer);
			loadingTimer = null;
		}
		loading = false;
		loadingSlow = false;
	}

	expanded = !expanded;
	onSelect(rootId, entry.path, true);
}

function handleDragOver(e: DragEvent) {
	if (!entry.isDir) return;
	e.preventDefault();
	e.stopPropagation();
	dragOver = true;
	if (e.dataTransfer) {
		e.dataTransfer.dropEffect = e.altKey ? 'copy' : 'move';
	}
}

function handleDragLeave() {
	dragOver = false;
}

async function handleDrop(e: DragEvent) {
	e.preventDefault();
	e.stopPropagation();
	dragOver = false;

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
			dstPath: entry.path,
		});
		onRefresh?.();
	} catch {
		// ignore
	}
}
</script>

<div class="select-none">
	<button
		class="flex min-w-0 w-full items-center gap-1 rounded px-1 py-0.5 text-left text-sm hover:bg-gray-700
		{isSelected ? 'bg-gray-700 text-white' : 'text-gray-300'}
		{dragOver ? 'ring-2 ring-blue-500 bg-blue-900/20' : ''}"
		style="padding-left: {depth * 16 + 4}px"
		onclick={toggle}
		ondragover={handleDragOver}
		ondragleave={handleDragLeave}
		ondrop={handleDrop}
	>
		{#if entry.isDir}
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
			<FileIcon {entry} open={expanded} size={16} />
		</span>
		<span class="truncate">{entry.name}</span>
		{#if loadingSlow}
			<span class="ml-1 shrink-0 text-xs text-amber-400">(network drive...)</span>
		{/if}
	</button>

	{#if expanded && children.length > 0}
		{#each children as child (child.path)}
			<TreeNode
				{rootId}
				entry={child}
				depth={depth + 1}
				{selectedPath}
				{onSelect}
				{onRefresh}
			/>
		{/each}
	{/if}
</div>

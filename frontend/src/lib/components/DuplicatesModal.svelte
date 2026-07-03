<script lang="ts">
import { deleteFiles, findDuplicates, thumbnailUrl } from '$lib/api';
import type { DuplicateFile, DuplicateGroup } from '$lib/types';
import { formatSize } from '$lib/utils';
import { ImageOff, RotateCw, X } from '@lucide/svelte';
import ConfirmDialog from './ConfirmDialog.svelte';

interface Props {
	rootId: string;
	path: string;
	onClose: () => void;
	onDeleted?: () => void;
}

let { rootId, path, onClose, onDeleted }: Props = $props();

let loading = $state(true);
let groups = $state<DuplicateGroup[]>([]);
let thumbErrors = $state<Record<string, boolean>>({});
let selected = $state<Record<string, boolean>>({});
let showConfirm = $state(false);
let deleting = $state(false);
let error = $state('');

function fileKey(file: DuplicateFile): string {
	return `${file.rootId}:${file.path}`;
}

function fileName(p: string): string {
	return p.split('/').pop() || p;
}

// Default selection: keep the first copy in each group, mark the rest.
function autoSelect() {
	const next: Record<string, boolean> = {};
	for (const group of groups) {
		for (const [i, file] of group.files.entries()) {
			next[fileKey(file)] = i > 0;
		}
	}
	selected = next;
}

function load() {
	loading = true;
	error = '';
	findDuplicates(rootId, path || '/')
		.then((r) => {
			groups = r.groups || [];
			autoSelect();
		})
		.catch(() => {
			groups = [];
		})
		.finally(() => {
			loading = false;
		});
}

$effect(() => {
	// Re-run when the scope changes.
	void rootId;
	void path;
	load();
});

let selectedFiles = $derived(
	groups.flatMap((g) =>
		g.files
			.filter((f) => selected[fileKey(f)])
			.map((f) => ({ file: f, size: g.size })),
	),
);
let selectedCount = $derived(selectedFiles.length);
let reclaimSize = $derived(selectedFiles.reduce((sum, s) => sum + s.size, 0));

function groupReclaim(group: DuplicateGroup): number {
	return group.size * (group.files.length - 1);
}

function clearSelection() {
	selected = {};
}

async function handleDelete() {
	showConfirm = false;
	deleting = true;
	error = '';
	// Duplicates can span roots — delete per root.
	const byRoot = new Map<string, string[]>();
	for (const { file } of selectedFiles) {
		const list = byRoot.get(file.rootId) || [];
		list.push(file.path);
		byRoot.set(file.rootId, list);
	}
	try {
		for (const [root, paths] of byRoot) {
			await deleteFiles(root, paths);
		}
		onDeleted?.();
		onClose();
	} catch (e) {
		error = e instanceof Error ? e.message : 'Delete failed';
		deleting = false;
		load();
	}
}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
	class="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
	onclick={onClose}
>
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div
		class="flex max-h-[88vh] w-[860px] max-w-full flex-col rounded-xl border border-gray-700 bg-gray-900 shadow-2xl"
		data-testid="duplicates-modal"
		onclick={(e) => e.stopPropagation()}
	>
		<div class="flex items-center gap-3 border-b border-gray-800 px-5 py-4">
			<h2 class="text-base font-semibold text-gray-100">Duplicates</h2>
			<span
				class="rounded-md border border-gray-800 bg-gray-800/70 px-2 py-0.5 font-mono text-xs text-gray-400"
			>
				{rootId}:{path || '/'}
			</span>
			<span class="text-xs text-gray-600">files in this folder that also exist elsewhere</span>
			<div class="flex-1"></div>
			<button
				class="text-gray-500 hover:text-gray-300"
				title="Rescan"
				onclick={load}
			>
				<RotateCw size={16} />
			</button>
			<button class="text-gray-500 hover:text-gray-300" title="Close" onclick={onClose}>
				<X size={18} />
			</button>
		</div>

		<div class="flex-1 space-y-4 overflow-y-auto p-5">
			{#if loading}
				<p class="text-center text-gray-500">Searching for duplicates...</p>
			{:else if groups.length === 0}
				<p class="text-center text-gray-500">No duplicates found</p>
			{:else}
				{#each groups as group (group.hash)}
					<div class="overflow-hidden rounded-lg border border-gray-800 bg-gray-800/50">
						<div
							class="flex flex-wrap items-center gap-2.5 border-b border-gray-800 px-3.5 py-2.5 text-xs"
						>
							<span
								class="rounded-full bg-emerald-400/15 px-2 py-0.5 font-semibold tracking-wide text-emerald-400"
							>
								EXACT
							</span>
							<span class="font-mono text-gray-600">{group.hashType} {group.hash.slice(0, 16)}…</span>
							<span class="text-gray-400">{group.files.length} files · {formatSize(group.size)} each</span>
							<span class="ml-auto font-semibold text-emerald-400">
								reclaim {formatSize(groupReclaim(group))}
							</span>
						</div>
						{#each group.files as file, i (fileKey(file))}
							<div
								class="grid grid-cols-[28px_72px_1fr_auto] items-center gap-3 border-b border-gray-800 px-3.5 py-2.5 last:border-b-0
								{selected[fileKey(file)] ? ' bg-red-400/10' : ''}"
							>
								<input
									type="checkbox"
									class="h-[15px] w-[15px] cursor-pointer accent-red-400"
									checked={selected[fileKey(file)] ?? false}
									onchange={() => {
										selected[fileKey(file)] = !selected[fileKey(file)];
									}}
								/>
								<div
									class="flex h-[54px] w-[72px] shrink-0 items-center justify-center overflow-hidden rounded-md border border-gray-800 bg-gray-800"
								>
									{#if thumbErrors[fileKey(file)]}
										<ImageOff size={18} class="text-gray-600" />
									{:else}
										<img
											src={thumbnailUrl(file.rootId, file.path)}
											alt=""
											loading="lazy"
											class="h-full w-full object-cover"
											onerror={() => {
												thumbErrors[fileKey(file)] = true;
											}}
										/>
									{/if}
								</div>
								<div class="min-w-0">
									<div class="truncate text-[13px] text-gray-100">{fileName(file.path)}</div>
									<div class="truncate font-mono text-[11px] text-gray-600">
										<span class="text-blue-400">{file.rootId}</span>{file.path.startsWith('/')
											? ''
											: '/'}{file.path}
									</div>
								</div>
								<div class="flex flex-col items-end gap-1">
									{#if i === 0 && !selected[fileKey(file)]}
										<span
											class="rounded border border-emerald-400/40 px-1.5 text-[10px] font-bold tracking-wider text-emerald-400"
										>
											KEEP
										</span>
									{:else}
										<span
											class="rounded bg-emerald-400/10 px-1.5 py-px font-mono text-[11px] text-emerald-400"
										>
											identical
										</span>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{/each}
			{/if}
		</div>

		<div
			class="flex items-center gap-3 rounded-b-xl border-t border-gray-800 bg-black/20 px-5 py-3.5"
		>
			<span class="text-[13px] text-gray-400">
				{#if error}
					<span class="text-red-400">{error}</span>
				{:else if selectedCount === 0}
					Nothing selected
				{:else}
					<strong class="text-gray-100">{selectedCount} file{selectedCount === 1 ? '' : 's'}</strong>
					marked for deletion ·
					<span class="font-semibold text-emerald-400">reclaim {formatSize(reclaimSize)}</span>
				{/if}
			</span>
			<div class="flex-1"></div>
			<button
				class="rounded-lg border border-gray-700 bg-gray-800 px-4 py-1.5 text-[13px] text-gray-400 hover:text-gray-100"
				onclick={clearSelection}
			>
				Clear selection
			</button>
			<button
				class="rounded-lg border border-gray-700 bg-gray-800 px-4 py-1.5 text-[13px] text-gray-400 hover:text-gray-100"
				title="Re-apply keeper suggestions"
				onclick={autoSelect}
			>
				Auto-select
			</button>
			<button
				class="rounded-lg bg-red-700 px-4 py-1.5 text-[13px] font-semibold text-white hover:bg-red-600 disabled:cursor-default disabled:opacity-40"
				disabled={selectedCount === 0 || deleting}
				onclick={() => {
					showConfirm = true;
				}}
			>
				{deleting
					? 'Deleting…'
					: selectedCount === 0
						? 'Delete…'
						: `Delete ${selectedCount} file${selectedCount === 1 ? '' : 's'}…`}
			</button>
		</div>
	</div>
</div>

{#if showConfirm}
	<ConfirmDialog
		title="Delete {selectedCount} duplicate{selectedCount === 1 ? '' : 's'}?"
		items={selectedFiles.map((s) => `${s.file.rootId}:${s.file.path}`)}
		onConfirm={handleDelete}
		onCancel={() => {
			showConfirm = false;
		}}
	/>
{/if}

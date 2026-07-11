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

type Tab = 'all' | 'sha256' | 'phash';

let loading = $state(true);
let groups = $state<DuplicateGroup[]>([]);
let thumbErrors = $state<Record<string, boolean>>({});
let selected = $state<Record<string, boolean>>({});
let tab = $state<Tab>('all');
let threshold = $state(88);
let showConfirm = $state(false);
let deleting = $state(false);
let error = $state('');
let delTotal = $state(0);
let delDone = $state(0);
let delCurrent = $state('');
let aborted = $state(false);

function fileKey(file: DuplicateFile): string {
	return `${file.rootId}:${file.path}`;
}

function isExact(group: DuplicateGroup): boolean {
	return group.hashType === 'sha256' || group.hashType === 'crc32';
}

function fileName(p: string): string {
	return p.split('/').pop() || p;
}

function fileSize(group: DuplicateGroup, file: DuplicateFile): number {
	return file.size ?? group.size;
}

function isKeeper(group: DuplicateGroup, index: number): boolean {
	return group.keeper != null && group.keeper === index;
}

function isTie(group: DuplicateGroup): boolean {
	return group.hashType === 'phash' && group.keeper == null;
}

// Default selection: mark everything except the suggested keeper. Quality
// ties (no keeper) are skipped — the user must pick which file to keep.
function autoSelect() {
	const next: Record<string, boolean> = {};
	for (const group of groups) {
		if (isTie(group)) continue;
		const keeper = group.keeper ?? 0;
		for (const [i, file] of group.files.entries()) {
			next[fileKey(file)] = i !== keeper;
		}
	}
	selected = next;
}

// Resolves a quality tie: keep the chosen file, mark the rest.
function keepThis(group: DuplicateGroup, index: number) {
	for (const [i, file] of group.files.entries()) {
		selected[fileKey(file)] = i !== index;
	}
}

function load() {
	loading = true;
	error = '';
	findDuplicates(rootId, path || '/', { similar: true, threshold })
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

let visibleGroups = $derived(
	tab === 'all'
		? groups
		: tab === 'sha256'
			? groups.filter((g) => isExact(g))
			: groups.filter((g) => g.hashType === tab),
);
let exactCount = $derived(groups.filter((g) => isExact(g)).length);
let similarCount = $derived(
	groups.filter((g) => g.hashType === 'phash').length,
);

let selectedFiles = $derived(
	groups.flatMap((g) =>
		g.files
			.filter((f) => selected[fileKey(f)])
			.map((f) => ({ file: f, size: fileSize(g, f) })),
	),
);
let selectedCount = $derived(selectedFiles.length);
let reclaimSize = $derived(selectedFiles.reduce((sum, s) => sum + s.size, 0));

function groupReclaim(group: DuplicateGroup): number {
	if (isExact(group)) return group.size * (group.files.length - 1);
	const keeper = group.keeper ?? null;
	if (keeper == null) {
		// Quality tie: best case keeps the largest file.
		const sizes = group.files.map((f) => fileSize(group, f));
		return sizes.reduce((a, b) => a + b, 0) - Math.max(...sizes);
	}
	return group.files.reduce(
		(sum, f, i) => (i === keeper ? sum : sum + fileSize(group, f)),
		0,
	);
}

function formatMtime(mtime?: number): string {
	if (!mtime) return '';
	return new Date(mtime * 1000).toISOString().slice(0, 10);
}

function clearSelection() {
	selected = {};
}

async function handleDelete() {
	showConfirm = false;
	deleting = true;
	aborted = false;
	delDone = 0;
	delCurrent = '';
	error = '';

	const targets = selectedFiles.map((s) => s.file);
	delTotal = targets.length;
	let failures = 0;

	for (const f of targets) {
		if (aborted) break;
		delCurrent = `${f.rootId}:${f.path}`;
		try {
			await deleteFiles(f.rootId, [f.path]);
		} catch {
			failures++;
		}
		delDone += 1;
	}

	if (aborted) {
		error = `Stopped after ${delDone} of ${delTotal}`;
		deleting = false;
		load();
		return;
	}

	if (failures > 0) {
		error = `${failures} of ${delTotal} deletions failed`;
		deleting = false;
		load();
		return;
	}

	onDeleted?.();
	onClose();
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
		class="flex max-h-[88vh] w-[860px] max-w-full flex-col rounded-xl border border-muted bg-surface shadow-2xl"
		data-testid="duplicates-modal"
		onclick={(e) => e.stopPropagation()}
	>
		<div class="flex items-center gap-3 border-b border-raised px-5 py-4">
			<h2 class="text-base font-semibold text-bright">Duplicates</h2>
			<span
				class="rounded-md border border-raised bg-raised/70 px-2 py-0.5 font-mono text-xs text-dim"
			>
				{rootId}:{path || '/'}
			</span>
			<span class="text-xs text-ghost">files in this folder that also exist elsewhere</span>
			<div class="flex-1"></div>
			<button
				class="text-faint hover:text-body"
				title="Rescan"
				onclick={load}
			>
				<RotateCw size={16} />
			</button>
			<button class="text-faint hover:text-body" title="Close" onclick={onClose}>
				<X size={18} />
			</button>
		</div>

		<div class="flex flex-wrap items-center gap-4 border-b border-raised px-5 py-3">
			<div class="flex gap-1">
				{#each [{ id: 'all', label: 'All', count: groups.length }, { id: 'sha256', label: 'Exact', count: exactCount }, { id: 'phash', label: 'Similar', count: similarCount }] as t (t.id)}
					<button
						class="flex items-center gap-1.5 rounded-full border px-3 py-1 text-[13px]
						{tab === t.id
							? ' border-accent-hover bg-accent-hover/15 text-bright'
							: ' border-transparent text-dim hover:text-body'}"
						onclick={() => {
							tab = t.id as Tab;
						}}
					>
						{t.label}
						<span class="rounded-full bg-raised px-1.5 text-[11px] text-faint">{t.count}</span>
					</button>
				{/each}
			</div>
			<div class="flex-1"></div>
			<label class="flex items-center gap-2 text-xs text-dim">
				Similarity ≥
				<input
					type="range"
					min="80"
					max="100"
					class="w-[110px] accent-accent-hover"
					bind:value={threshold}
					onchange={load}
				/>
				<span class="w-[3.5em] font-mono text-bright">{threshold}%</span>
			</label>
		</div>

		<div class="flex-1 space-y-4 overflow-y-auto p-5">
			{#if loading}
				<p class="text-center text-faint">Searching for duplicates...</p>
			{:else if visibleGroups.length === 0}
				<p class="text-center text-faint">No duplicates found</p>
			{:else}
				{#each visibleGroups as group (group.hash)}
					<div class="overflow-hidden rounded-lg border border-raised bg-raised/50">
						<div
							class="flex flex-wrap items-center gap-2.5 border-b border-raised px-3.5 py-2.5 text-xs"
						>
							{#if group.hashType === 'phash'}
								<span
									class="rounded-full bg-tag-soft/15 px-2 py-0.5 font-semibold tracking-wide text-tag-soft"
								>
									SIMILAR
								</span>
								{#if isTie(group)}
									<span
										class="rounded-full bg-danger-soft/15 px-2 py-0.5 font-semibold tracking-wide text-danger-soft"
									>
										QUALITY TIE
									</span>
								{/if}
								<span class="font-mono text-ghost">
									phash {group.hash.slice(0, 8)}… ±{group.maxDistance ?? 0} bits
								</span>
								<span class="text-faint">
									{isTie(group) ? 'same resolution → manual pick' : 'keeper: highest resolution'}
								</span>
							{:else}
								<span
									class="rounded-full bg-ok/15 px-2 py-0.5 font-semibold tracking-wide text-ok"
								>
									EXACT
								</span>
								<span class="font-mono text-ghost">{group.hashType} {group.hash.slice(0, 16)}…</span>
								<span class="text-dim">{group.files.length} files · {formatSize(group.size)} each</span>
							{/if}
							<span class="ml-auto font-semibold text-ok">
								reclaim {isTie(group) ? 'up to ' : ''}{formatSize(groupReclaim(group))}
							</span>
						</div>
						{#each group.files as file, i (fileKey(file))}
							<div
								class="grid grid-cols-[28px_72px_1fr_auto] items-center gap-3 border-b border-raised px-3.5 py-2.5 last:border-b-0
								{selected[fileKey(file)] ? ' bg-danger-soft/10' : ''}"
							>
								<input
									type="checkbox"
									class="h-[15px] w-[15px] cursor-pointer accent-danger-soft"
									checked={selected[fileKey(file)] ?? false}
									onchange={() => {
										selected[fileKey(file)] = !selected[fileKey(file)];
									}}
								/>
								<div
									class="flex h-[54px] w-[72px] shrink-0 items-center justify-center overflow-hidden rounded-md border border-raised bg-raised"
								>
									{#if thumbErrors[fileKey(file)]}
										<ImageOff size={18} class="text-ghost" />
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
									<div class="truncate text-[13px] text-bright">{fileName(file.path)}</div>
									<div class="truncate font-mono text-[11px] text-ghost">
										<span class="text-accent-soft">{file.rootId}</span>{file.path.startsWith('/')
											? ''
											: '/'}{file.path}
									</div>
									{#if file.size || file.width}
										<div class="flex flex-wrap gap-2.5 text-[11px] text-dim">
											{#if file.size}<span>{formatSize(file.size)}</span>{/if}
											{#if file.width && file.height}
												<span class={isKeeper(group, i) ? 'text-ok' : ''}>
													{file.width}×{file.height}{isKeeper(group, i) && group.hashType === 'phash'
														? ' ← best'
														: ''}
												</span>
											{/if}
											{#if file.mtime}<span>{formatMtime(file.mtime)}</span>{/if}
										</div>
									{/if}
								</div>
								<div class="flex flex-col items-end gap-1">
									{#if isTie(group)}
										{#if selected[fileKey(file)] === false && group.files.some((f) => selected[fileKey(f)])}
											<span
												class="rounded border border-ok/40 px-1.5 text-[10px] font-bold tracking-wider text-ok"
											>
												✓ KEEPING
											</span>
										{:else}
											<button
												class="rounded border border-dashed border-muted px-1.5 text-[10px] font-bold tracking-wider text-faint hover:border-ok hover:text-ok"
												onclick={() => keepThis(group, i)}
											>
												KEEP THIS
											</button>
										{/if}
									{:else if isKeeper(group, i) && !selected[fileKey(file)]}
										<span
											class="rounded border border-ok/40 px-1.5 text-[10px] font-bold tracking-wider text-ok"
										>
											KEEP
										</span>
									{:else if group.hashType === 'phash'}
										<span
											class="rounded bg-tag-soft/10 px-1.5 py-px font-mono text-[11px] text-tag-soft"
										>
											{file.similarity ?? 0}% match
										</span>
									{:else}
										<span
											class="rounded bg-ok/10 px-1.5 py-px font-mono text-[11px] text-ok"
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
			class="flex flex-col gap-3 rounded-b-xl border-t border-raised bg-black/20 px-5 py-3.5"
		>
			{#if deleting}
				<!-- Progress bar -->
				<div class="flex flex-col gap-1.5">
					<div class="flex items-center justify-between text-xs text-dim">
						<span>Deleting files…</span>
						<span class="tabular-nums">{delDone} / {delTotal}</span>
					</div>
					<div class="h-2 w-full overflow-hidden rounded-full bg-raised">
						<div
							class="h-full rounded-full bg-accent transition-all duration-200"
							style="width: {delTotal ? (delDone / delTotal) * 100 : 0}%"
						></div>
					</div>
					{#if delCurrent}
						<p class="truncate font-mono text-xs text-faint">{delCurrent}</p>
					{/if}
				</div>
			{/if}
			<div class="flex items-center gap-3">
				<span class="text-[13px] text-dim">
					{#if error}
						<span class="text-danger-soft">{error}</span>
					{:else if selectedCount === 0}
						Nothing selected
					{:else}
						<strong class="text-bright">{selectedCount} file{selectedCount === 1 ? '' : 's'}</strong>
						marked for deletion ·
						<span class="font-semibold text-ok">reclaim {formatSize(reclaimSize)}</span>
					{/if}
				</span>
				<div class="flex-1"></div>
				{#if deleting}
					<button
						class="rounded-lg border border-muted bg-raised px-4 py-1.5 text-[13px] text-dim hover:text-bright"
						onclick={() => { aborted = true; }}
					>
						Stop
					</button>
				{:else}
					<button
						class="rounded-lg border border-muted bg-raised px-4 py-1.5 text-[13px] text-dim hover:text-bright"
						onclick={clearSelection}
					>
						Clear selection
					</button>
					<button
						class="rounded-lg border border-muted bg-raised px-4 py-1.5 text-[13px] text-dim hover:text-bright"
						title="Re-apply keeper suggestions"
						onclick={autoSelect}
					>
						Auto-select
					</button>
				{/if}
				<button
					class="rounded-lg bg-danger-strong px-4 py-1.5 text-[13px] font-semibold text-white hover:bg-danger disabled:cursor-default disabled:opacity-40"
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
</div>

{#if showConfirm}
	<ConfirmDialog
		title="Delete {selectedCount} duplicate{selectedCount === 1 ? '' : 's'}?"
		items={selectedFiles.map((s) => ({
			name: `${s.file.rootId}:${s.file.path}`,
			thumbUrl: thumbnailUrl(s.file.rootId, s.file.path),
		}))}
		onConfirm={handleDelete}
		onCancel={() => {
			showConfirm = false;
		}}
	/>
{/if}

<script lang="ts">
import { fetchDirectory, moveFiles, getOrganizePerformers } from '$lib/api';
import type { OrganizeGroup, OrganizeFile, ItemResult } from '$lib/api';
import type { FileEntry } from '$lib/types';
import { sortEntries } from '$lib/utils';
import TreeNode from './TreeNode.svelte';
import { ChevronDown, ChevronRight, FolderInput, X } from '@lucide/svelte';

interface Props {
	rootId: string;
	archiveRootId: string;
	groups: OrganizeGroup[];
	unmatched: OrganizeFile[];
	onClose: () => void;
	onMoved?: (results: ItemResult[]) => void;
	onToast?: (msg: string, kind: 'success' | 'error') => void;
}

let {
	rootId,
	archiveRootId,
	groups,
	unmatched,
	onClose,
	onMoved,
	onToast,
}: Props = $props();

// Mutable local copies of group state so the tree picker can override targetPath reactively.
interface LocalGroup {
	performer: string;
	targetRoot: string;
	targetPath: string;
	exists: boolean;
	matchedBy: 'filename' | 'dirname';
	candidates: string[];
	files: OrganizeFile[];
}

let localGroups = $state<LocalGroup[]>([]);
let checked = $state<boolean[]>([]);
let expanded = $state<boolean[]>([]);
let pickerExpanded = $state<boolean[]>([]);
let applying = $state(false);
let moveTotal = $state(0);
let moveDone = $state(0);
let moveCurrent = $state('');
let aborted = $state(false);

// Archive root top-level dirs for the tree picker.
let archiveDirs = $state<FileEntry[]>([]);
let archiveDirsLoaded = $state(false);

// All performer names from Stash, loaded lazily for the override combobox.
let allPerformers = $state<string[]>([]);
let allPerformersLoaded = $state(false);
// Track which groups have the performer override input expanded.
let performerOverrideExpanded = $state<boolean[]>([]);

$effect.pre(() => {
	// Run once on mount to initialise mutable state from props.
	if (localGroups.length === 0 && groups.length > 0) {
		localGroups = groups.map((g) => ({ ...g, candidates: g.candidates ?? [] }));
		checked = groups.map(() => true);
		expanded = groups.map(() => false);
		pickerExpanded = groups.map(() => false);
		performerOverrideExpanded = groups.map(() => false);
	}
});

// Load archive root dirs lazily when first picker is opened.
async function ensureArchiveDirs() {
	if (archiveDirsLoaded) return;
	archiveDirsLoaded = true;
	try {
		const entries = await fetchDirectory(archiveRootId, '/');
		archiveDirs = sortEntries(entries).filter((e) => e.isDir);
	} catch {
		archiveDirs = [];
	}
}

// Load all performer names lazily the first time any override control is opened.
async function ensureAllPerformers() {
	if (allPerformersLoaded) return;
	allPerformersLoaded = true;
	try {
		allPerformers = await getOrganizePerformers();
	} catch {
		allPerformers = [];
	}
}

function togglePerformerOverride(i: number) {
	performerOverrideExpanded[i] = !performerOverrideExpanded[i];
	if (performerOverrideExpanded[i]) {
		void ensureAllPerformers();
	}
}

function handlePerformerOverride(i: number, val: string) {
	const trimmed = val.trim();
	if (!trimmed) return;

	// Case-insensitive match to find the canonical-cased name.
	const lower = trimmed.toLowerCase();
	const canonical = allPerformers.find((p) => p.toLowerCase() === lower);
	if (!canonical) {
		// Not a known performer — revert by leaving state unchanged; the input
		// will be reset by Svelte on next render via the value binding.
		return;
	}

	const g = localGroups[i];
	g.performer = canonical;

	// Load archive dirs if not already done so we can look up an existing folder.
	void ensureArchiveDirs().then(() => {
		// Find a top-level archive dir whose name matches the chosen performer.
		const match = archiveDirs.find(
			(d) => d.name.toLowerCase() === canonical.toLowerCase(),
		);
		if (match) {
			g.targetPath = match.name;
			g.exists = true;
		} else {
			g.targetPath = canonical;
			g.exists = false;
		}
		// Clear auto-candidate list — this is a manual pick.
		g.candidates = [];
	});
}

function togglePicker(i: number) {
	pickerExpanded[i] = !pickerExpanded[i];
	if (pickerExpanded[i]) {
		void ensureArchiveDirs();
	}
}

function handleSelectDir(
	i: number,
	_rootId: string,
	path: string,
	isDir: boolean,
) {
	if (!isDir) return;
	const g = localGroups[i];
	// path is root-relative; strip any leading slash (e.g. "Videos/Vanessa Serros").
	const base = path.replace(/^\//, '');
	if (base === '') {
		// Archive root selected: create a performer-named dir at the top level.
		g.targetPath = g.performer;
		g.exists = false;
		return;
	}
	const leaf = base.slice(base.lastIndexOf('/') + 1);
	if (leaf.toLowerCase() === g.performer.toLowerCase()) {
		// The picked dir IS the performer's folder — use it directly.
		g.targetPath = base;
		g.exists = true;
	} else {
		// Treat the picked dir as a base; nest a performer-named subdir under it.
		g.targetPath = `${base}/${g.performer}`;
		g.exists = false;
	}
}

// True when a file already lives in its target directory (same root + same
// parent dir), so moving it would be a self-move — skip it entirely.
function isSelfTarget(g: LocalGroup, filePath: string): boolean {
	if (g.targetRoot !== rootId) return false;
	const slash = filePath.lastIndexOf('/');
	const parent = slash >= 0 ? filePath.slice(0, slash) : '';
	return parent.replace(/^\//, '') === g.targetPath.replace(/^\//, '');
}

async function handleApply() {
	applying = true;
	aborted = false;
	moveDone = 0;
	moveCurrent = '';
	moveTotal = 0;
	for (let i = 0; i < localGroups.length; i++) {
		if (!checked[i]) continue;
		const g = localGroups[i];
		for (const file of g.files) {
			if (!isSelfTarget(g, file.path)) moveTotal += 1;
		}
	}
	try {
		const allResults: ItemResult[] = [];
		outer: for (let i = 0; i < localGroups.length; i++) {
			if (!checked[i]) continue;
			const g = localGroups[i];
			for (const file of g.files) {
				if (aborted) break outer;
				if (isSelfTarget(g, file.path)) continue; // already in place
				moveCurrent = `${g.targetPath}/${file.name}`;
				try {
					const results = await moveFiles({
						items: [{ srcRoot: rootId, srcPath: file.path }],
						dstRoot: g.targetRoot,
						dstPath: g.targetPath,
						createDst: true,
					});
					allResults.push(...results);
				} catch (err) {
					// Collect per-file errors as a synthetic result rather than aborting the whole batch.
					allResults.push({
						srcRoot: rootId,
						srcPath: file.path,
						error: err instanceof Error ? err.message : String(err),
					});
				}
				moveDone += 1;
			}
		}

		if (aborted) {
			onToast?.(`Stopped after ${moveDone} of ${moveTotal} file(s)`, 'error');
			// Keep modal open so the user can see what happened.
			return;
		}

		const errors = allResults.filter((r) => r.error);
		if (errors.length === 0) {
			onToast?.(`Moved ${allResults.length} file(s) successfully`, 'success');
		} else {
			onToast?.(
				`Moved ${allResults.length - errors.length} file(s); ${errors.length} failed`,
				'error',
			);
		}
		onMoved?.(allResults);
		onClose();
	} catch (err) {
		onToast?.(
			`Move failed: ${err instanceof Error ? err.message : String(err)}`,
			'error',
		);
	} finally {
		applying = false;
	}
}

function handleKeydown(e: KeyboardEvent) {
	if (e.key === 'Escape') onClose();
}

function totalCheckedFiles(): number {
	let count = 0;
	for (let i = 0; i < localGroups.length; i++) {
		if (checked[i]) count += localGroups[i].files.length;
	}
	return count;
}
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
	onclick={onClose}
>
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="mx-4 flex w-full max-w-2xl flex-col rounded-xl border border-gray-700 bg-gray-900 shadow-2xl"
		style="max-height: 90vh;"
		onclick={(e) => e.stopPropagation()}
	>
		<!-- Header -->
		<div class="flex items-center justify-between border-b border-gray-700 px-6 py-4">
			<div class="flex items-center gap-3">
				<div class="flex h-9 w-9 items-center justify-center rounded-full bg-blue-900/50">
					<FolderInput size={18} class="text-blue-400" />
				</div>
				<div>
					<h2 class="text-base font-semibold text-gray-100">Organize inbox</h2>
					<p class="text-xs text-gray-500">
						{localGroups.length} performer group(s) matched — {unmatched.length} unmatched
					</p>
				</div>
			</div>
			<button
				class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300"
				onclick={onClose}
			>
				<X size={16} />
			</button>
		</div>

		<!-- Shared datalist for performer comboboxes -->
		<datalist id="organize-performers">
			{#each allPerformers as name}
				<option value={name}></option>
			{/each}
		</datalist>

		<!-- Body -->
		<div class="flex-1 overflow-y-auto themed-scroll px-6 py-4 space-y-3">
			{#if localGroups.length === 0 && unmatched.length === 0}
				<p class="text-sm text-gray-500">No files found in this directory.</p>
			{/if}

			<!-- Matched groups -->
			{#each localGroups as group, i}
				<div class="rounded-lg border border-gray-700 bg-gray-800/50">
					<div class="flex items-center gap-3 px-4 py-3">
						<input
							type="checkbox"
							class="h-4 w-4 rounded accent-blue-500"
							bind:checked={checked[i]}
						/>
						<button
							class="flex flex-1 items-center gap-2 text-left"
							onclick={() => (expanded[i] = !expanded[i])}
						>
							{#if expanded[i]}
								<ChevronDown size={14} class="shrink-0 text-gray-500" />
							{:else}
								<ChevronRight size={14} class="shrink-0 text-gray-500" />
							{/if}
							<span class="font-medium text-gray-200">{group.performer}</span>
							<span class="text-xs text-gray-500">→ {group.targetPath}</span>
							<span class="ml-auto text-xs text-gray-500">{group.files.length} file(s)</span>
						</button>
						<!-- exists badge -->
						<span
							class="rounded px-1.5 py-0.5 text-xs font-medium {group.exists
								? 'bg-emerald-900/50 text-emerald-400'
								: 'bg-amber-900/50 text-amber-400'}"
						>
							{group.exists ? 'existing' : 'new'}
						</span>
						<!-- matchedBy badge -->
						<span
							class="rounded px-1.5 py-0.5 text-xs font-medium {group.matchedBy === 'filename'
								? 'bg-emerald-900/50 text-emerald-400'
								: 'bg-amber-900/50 text-amber-400'}"
						>
							{group.matchedBy}
						</span>
					</div>

					<!-- Candidate chooser: shown when multiple archive dirs matched the performer -->
					{#if group.candidates.length > 1}
						<div class="border-t border-gray-700/50 px-4 py-2">
							<p class="mb-1.5 text-xs font-medium text-gray-500 uppercase tracking-wide">Select target folder</p>
							<div class="space-y-1">
								{#each group.candidates as candidate}
									<label class="flex items-center gap-2 cursor-pointer rounded px-2 py-1 hover:bg-gray-700/40 transition-colors">
										<input
											type="radio"
											name="candidate-{i}"
											value={candidate}
											class="accent-blue-500"
											checked={group.targetPath === candidate}
											onchange={() => { localGroups[i].targetPath = candidate; localGroups[i].exists = true; }}
										/>
										<span class="text-xs text-gray-300 font-mono">{candidate}</span>
									</label>
								{/each}
							</div>
						</div>
					{/if}

					<!-- Performer override -->
					<div class="border-t border-gray-700/50 px-4 py-2">
						<button
							class="flex items-center gap-1 text-xs text-gray-500 hover:text-gray-300 transition-colors"
							onclick={() => togglePerformerOverride(i)}
						>
							{#if performerOverrideExpanded[i]}
								<ChevronDown size={12} />
							{:else}
								<ChevronRight size={12} />
							{/if}
							Change performer
						</button>
						{#if performerOverrideExpanded[i]}
							<div class="mt-2">
								<input
									type="text"
									list="organize-performers"
									class="w-full bg-gray-800 border border-gray-700 text-gray-200 text-xs rounded px-2 py-1 focus:outline-none focus:border-blue-600"
									placeholder="Type to search performers…"
									value={group.performer}
									onchange={(e) => {
										handlePerformerOverride(i, (e.target as HTMLInputElement).value);
										// Revert input to current (canonical) performer name.
										(e.target as HTMLInputElement).value = localGroups[i].performer;
									}}
								/>
							</div>
						{/if}
					</div>

					<!-- Tree picker to override target dir -->
					<div class="border-t border-gray-700/50 px-4 py-2">
						<button
							class="flex items-center gap-1 text-xs text-gray-500 hover:text-gray-300 transition-colors"
							onclick={() => togglePicker(i)}
						>
							{#if pickerExpanded[i]}
								<ChevronDown size={12} />
							{:else}
								<ChevronRight size={12} />
							{/if}
							Override target folder
						</button>
						{#if pickerExpanded[i]}
							<div class="mt-2 max-h-36 overflow-y-auto themed-scroll rounded border border-gray-700 bg-gray-900 px-2 py-1">
								{#if archiveDirs.length === 0}
									<p class="py-1 text-xs text-gray-600">Loading...</p>
								{:else}
									{#each archiveDirs as dir (dir.path)}
										<TreeNode
											rootId={archiveRootId}
											entry={dir}
											depth={0}
											selectedPath={`${archiveRootId}:${group.targetPath}`}
											onSelect={(rid, path, isDir) => handleSelectDir(i, rid, path, isDir)}
										/>
									{/each}
								{/if}
							</div>
						{/if}
					</div>

					{#if expanded[i]}
						<div class="border-t border-gray-700 px-4 py-2">
							{#each group.files as file}
								<p class="truncate py-0.5 text-xs text-gray-400">{file.name}</p>
							{/each}
						</div>
					{/if}
				</div>
			{/each}

			<!-- Unmatched section -->
			{#if unmatched.length > 0}
				<div class="rounded-lg border border-gray-700/50 bg-gray-800/20 px-4 py-3">
					<p class="mb-2 text-xs font-medium text-gray-500 uppercase tracking-wide">
						Unmatched ({unmatched.length})
					</p>
					{#each unmatched as file}
						<p class="truncate py-0.5 text-xs text-gray-600">{file.name}</p>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Footer -->
		<div class="flex flex-col gap-3 border-t border-gray-700 px-6 py-4">
			{#if applying}
				<!-- Progress bar -->
				<div class="flex flex-col gap-1.5">
					<div class="flex items-center justify-between text-xs text-gray-400">
						<span>Moving files…</span>
						<span class="tabular-nums">{moveDone} / {moveTotal}</span>
					</div>
					<div class="h-2 w-full overflow-hidden rounded-full bg-gray-800">
						<div
							class="h-full rounded-full bg-blue-600 transition-all duration-200"
							style="width: {moveTotal ? (moveDone / moveTotal) * 100 : 0}%"
						></div>
					</div>
					{#if moveCurrent}
						<p class="truncate font-mono text-xs text-gray-500">{moveCurrent}</p>
					{/if}
				</div>
			{/if}
			<div class="flex items-center justify-between">
				<p class="text-xs text-gray-500">
					{#if applying}
						{moveDone} of {moveTotal} file(s) moved
					{:else}
						{totalCheckedFiles()} file(s) will be moved
					{/if}
				</p>
				<div class="flex gap-3">
					<button
						class="rounded-lg px-4 py-2 text-sm font-medium text-gray-300 transition-colors hover:bg-gray-800"
						onclick={applying ? () => { aborted = true; } : onClose}
					>
						{applying ? 'Stop' : 'Cancel'}
					</button>
					<button
						class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-500 disabled:cursor-not-allowed disabled:opacity-40"
						disabled={applying || totalCheckedFiles() === 0}
						onclick={handleApply}
					>
						{applying ? 'Moving…' : 'Move files'}
					</button>
				</div>
			</div>
		</div>
	</div>
</div>

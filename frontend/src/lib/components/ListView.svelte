<script lang="ts">
import type { FileEntry } from '$lib/types';
import { formatSize, formatDate } from '$lib/utils';
import FileIcon from './FileIcon.svelte';
import { ScanText, User } from '@lucide/svelte';
import { thumbnailUrl } from '$lib/api';

interface Props {
	rootId: string;
	entries: FileEntry[];
	isSelected: (path: string) => boolean;
	isCut: (path: string) => boolean;
	onclick: (entry: FileEntry, e: MouseEvent) => void;
	ondblclick: (entry: FileEntry) => void;
	oncontextmenu: (e: MouseEvent, entry: FileEntry) => void;
	ondragstart: (e: DragEvent, entry: FileEntry) => void;
}

let {
	rootId,
	entries,
	isSelected,
	isCut,
	onclick,
	ondblclick,
	oncontextmenu,
	ondragstart,
}: Props = $props();

type SortKey = 'name' | 'size' | 'modTime' | 'mediaType';
type SortDir = 'asc' | 'desc';

let sortKey = $state<SortKey>('name');
let sortDir = $state<SortDir>('asc');

let sortedEntries = $derived.by(() => {
	const sorted = [...entries];
	sorted.sort((a, b) => {
		// Directories always first
		if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;

		let cmp = 0;
		switch (sortKey) {
			case 'name':
				cmp = a.name.localeCompare(b.name);
				break;
			case 'size':
				cmp = a.size - b.size;
				break;
			case 'modTime':
				cmp = new Date(a.modTime).getTime() - new Date(b.modTime).getTime();
				break;
			case 'mediaType':
				cmp = a.mediaType.localeCompare(b.mediaType);
				break;
		}
		return sortDir === 'asc' ? cmp : -cmp;
	});
	return sorted;
});

function handleSort(key: SortKey) {
	if (sortKey === key) {
		sortDir = sortDir === 'asc' ? 'desc' : 'asc';
	} else {
		sortKey = key;
		sortDir = 'asc';
	}
}

function sortIndicator(key: SortKey): string {
	if (sortKey !== key) return '';
	return sortDir === 'asc' ? ' \u25B2' : ' \u25BC';
}

function ariaSort(key: SortKey): 'ascending' | 'descending' | undefined {
	if (sortKey !== key) return undefined;
	return sortDir === 'asc' ? 'ascending' : 'descending';
}
</script>

<div class="w-full">
	<table class="w-full text-sm text-left">
		<thead class="text-xs text-dim uppercase border-b border-muted sticky top-0 bg-canvas z-10">
			<tr>
				<th scope="col" class="w-10 px-2 py-2"><span class="sr-only">Preview</span></th>
				<th scope="col" class="p-0" aria-sort={ariaSort('name')}>
					<button
						type="button"
						class="w-full px-2 py-2 text-left uppercase cursor-pointer select-none hover:text-body"
						onclick={() => handleSort('name')}
					>
						Name{sortIndicator('name')}
					</button>
				</th>
				<th scope="col" class="p-0 w-24" aria-sort={ariaSort('size')}>
					<button
						type="button"
						class="w-full px-2 py-2 text-right uppercase cursor-pointer select-none hover:text-body"
						onclick={() => handleSort('size')}
					>
						Size{sortIndicator('size')}
					</button>
				</th>
				<th scope="col" class="p-0 w-48 whitespace-nowrap" aria-sort={ariaSort('modTime')}>
					<button
						type="button"
						class="w-full px-2 py-2 text-left uppercase cursor-pointer select-none hover:text-body"
						onclick={() => handleSort('modTime')}
					>
						Modified{sortIndicator('modTime')}
					</button>
				</th>
				<th scope="col" class="p-0 w-20" aria-sort={ariaSort('mediaType')}>
					<button
						type="button"
						class="w-full px-2 py-2 text-left uppercase cursor-pointer select-none hover:text-body"
						onclick={() => handleSort('mediaType')}
					>
						Type{sortIndicator('mediaType')}
					</button>
				</th>
				<th scope="col" class="px-2 py-2 w-32">Tags</th>
			</tr>
		</thead>
		<tbody>
			{#each sortedEntries as entry (entry.path)}
				<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
				<tr
					class="border-b border-raised/50 transition-colors cursor-pointer
					{isSelected(entry.path)
						? 'bg-accent-deep/30'
						: 'hover:bg-raised/50'}
					{isCut(entry.path) ? ' opacity-50' : ''}"
					tabindex={0}
					aria-selected={isSelected(entry.path)}
					draggable={true}
					onclick={(e) => { e.stopPropagation(); onclick(entry, e); }}
					ondblclick={() => ondblclick(entry)}
					oncontextmenu={(e) => oncontextmenu(e, entry)}
					ondragstart={(e) => ondragstart(e, entry)}
					onkeydown={(e) => {
						if (e.key === 'Enter') {
							e.preventDefault();
							ondblclick(entry);
						} else if (e.key === ' ') {
							// Space selects; preventDefault stops the page scrolling.
							e.preventDefault();
							onclick(entry, new MouseEvent('click'));
						}
					}}
				>
					<td class="px-2 py-1.5 text-faint">
						{#if !entry.isDir && entry.hasThumb && (entry.mediaType === 'image' || entry.mediaType === 'video')}
							<img
								src={thumbnailUrl(rootId, entry.path)}
								alt=""
								width="32"
								height="32"
								loading="lazy"
								class="h-8 w-8 rounded object-cover bg-raised"
							/>
						{:else}
							<FileIcon {entry} size={16} />
						{/if}
					</td>
					<td class="px-2 py-1.5 text-body">
						<div class="flex items-center gap-2">
							<span class="truncate" title={entry.name}>{entry.name}</span>
							{#if entry.ocrText}
								<span class="text-warn shrink-0" role="img" aria-label="Contains text" title="Contains text (OCR)"><ScanText size={12} aria-hidden="true" /></span>
							{/if}
							{#if entry.hasPerson === true}
								<span class="text-warn shrink-0" role="img" aria-label="Contains a person" title="Contains a person"><User size={12} aria-hidden="true" /></span>
							{/if}
						</div>
					</td>
					<td class="px-2 py-1.5 text-dim text-right tabular-nums">
						{entry.isDir ? '\u2014' : formatSize(entry.size)}
					</td>
					<td class="px-2 py-1.5 text-dim tabular-nums whitespace-nowrap">
						{formatDate(entry.modTime)}
					</td>
					<td class="px-2 py-1.5 text-dim">
						{entry.isDir ? 'folder' : entry.mediaType}
					</td>
					<td class="px-2 py-1.5">
						{#if entry.tags && entry.tags.length > 0}
							<div class="flex flex-wrap gap-0.5">
								{#each entry.tags.slice(0, 3) as tag}
									<span class="rounded bg-tag/80 px-1 py-0.5 text-[10px] font-medium leading-none text-white">
										{tag.label}
									</span>
								{/each}
							</div>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

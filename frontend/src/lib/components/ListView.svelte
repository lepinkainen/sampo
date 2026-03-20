<script lang="ts">
import type { FileEntry } from '$lib/types';
import { formatSize, formatDate } from '$lib/utils';
import FileIcon from './FileIcon.svelte';
import { User } from '@lucide/svelte';
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
</script>

<div class="w-full">
	<table class="w-full text-sm text-left">
		<thead class="text-xs text-gray-400 uppercase border-b border-gray-700 sticky top-0 bg-gray-950 z-10">
			<tr>
				<th class="w-10 px-2 py-2"></th>
				<th class="px-2 py-2 cursor-pointer select-none hover:text-gray-200" onclick={() => handleSort('name')}>
					Name{sortIndicator('name')}
				</th>
				<th class="px-2 py-2 cursor-pointer select-none hover:text-gray-200 w-24 text-right" onclick={() => handleSort('size')}>
					Size{sortIndicator('size')}
				</th>
				<th class="px-2 py-2 cursor-pointer select-none hover:text-gray-200 w-44" onclick={() => handleSort('modTime')}>
					Modified{sortIndicator('modTime')}
				</th>
				<th class="px-2 py-2 cursor-pointer select-none hover:text-gray-200 w-20" onclick={() => handleSort('mediaType')}>
					Type{sortIndicator('mediaType')}
				</th>
				<th class="px-2 py-2 w-32">Tags</th>
			</tr>
		</thead>
		<tbody>
			{#each sortedEntries as entry (entry.path)}
				<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
				<tr
					class="border-b border-gray-800/50 transition-colors cursor-pointer
					{isSelected(entry.path)
						? 'bg-blue-900/30'
						: 'hover:bg-gray-800/50'}
					{isCut(entry.path) ? ' opacity-50' : ''}"
					tabindex={0}
					draggable={true}
					onclick={(e) => { e.stopPropagation(); onclick(entry, e); }}
					ondblclick={() => ondblclick(entry)}
					oncontextmenu={(e) => oncontextmenu(e, entry)}
					ondragstart={(e) => ondragstart(e, entry)}
					onkeydown={(e) => {
						if (e.key === 'Enter') {
							e.preventDefault();
							ondblclick(entry);
						}
					}}
				>
					<td class="px-2 py-1.5 text-gray-500">
						<FileIcon {entry} size={16} />
					</td>
					<td class="px-2 py-1.5 text-gray-200">
						<div class="flex items-center gap-2">
							<span class="truncate" title={entry.name}>{entry.name}</span>
							{#if entry.hasPerson === true}
								<span class="text-orange-400 shrink-0"><User size={12} /></span>
							{/if}
						</div>
					</td>
					<td class="px-2 py-1.5 text-gray-400 text-right tabular-nums">
						{entry.isDir ? '\u2014' : formatSize(entry.size)}
					</td>
					<td class="px-2 py-1.5 text-gray-400 tabular-nums">
						{formatDate(entry.modTime)}
					</td>
					<td class="px-2 py-1.5 text-gray-400">
						{entry.isDir ? 'folder' : entry.mediaType}
					</td>
					<td class="px-2 py-1.5">
						{#if entry.tags && entry.tags.length > 0}
							<div class="flex flex-wrap gap-0.5">
								{#each entry.tags.slice(0, 3) as tag}
									<span class="rounded bg-purple-600/80 px-1 py-0.5 text-[10px] font-medium leading-none text-white">
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

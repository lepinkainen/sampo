<script lang="ts">
import type { FileEntry } from '$lib/types';
import {
	Folder,
	FolderOpen,
	FileArchive,
	Image as ImageIcon,
	Video,
	File,
	FileText,
} from '@lucide/svelte';

interface Props {
	entry: FileEntry;
	size: number;
	open?: boolean;
}

let { entry, size, open = false }: Props = $props();
</script>

{#if entry.isDir}
	{#if open}
		<FolderOpen {size} />
	{:else}
		<Folder {size} />
	{/if}
{:else if entry.isZip || entry.mediaType === 'archive'}
	<FileArchive {size} />
{:else if entry.mediaType === 'image'}
	<ImageIcon {size} />
{:else if entry.mediaType === 'video'}
	<Video {size} />
{:else if entry.mediaType === 'pdf'}
	<FileText {size} />
{:else}
	<File {size} />
{/if}

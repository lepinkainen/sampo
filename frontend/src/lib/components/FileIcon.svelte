<script lang="ts">
import type { FileEntry } from '$lib/types';
import {
	Folder,
	FileArchive,
	Image as ImageIcon,
	Video,
	File,
} from '@lucide/svelte';

interface Props {
	entry: FileEntry;
	size: number;
}

let { entry, size }: Props = $props();
</script>

{#if entry.isDir}
	<Folder {size} />
{:else if entry.isZip || entry.mediaType === 'archive'}
	<FileArchive {size} />
{:else if entry.mediaType === 'image'}
	<ImageIcon {size} />
{:else if entry.mediaType === 'video'}
	<Video {size} />
{:else}
	<File {size} />
{/if}

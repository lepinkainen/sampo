<script lang="ts">
import { goto } from '$app/navigation';
import { page } from '$app/stores';
import ThumbnailGrid from '$lib/components/ThumbnailGrid.svelte';
import TreeView from '$lib/components/TreeView.svelte';

// Derived state from URL
let selectedRootId = $derived($page.url.searchParams.get('root'));
let selectedPath = $derived($page.url.searchParams.get('path'));
let previewPath = $derived($page.url.searchParams.get('preview'));

// Tree selection key
let selectedKey = $derived(
	selectedRootId && selectedPath ? `${selectedRootId}:${selectedPath}` : null,
);

function updateUrl(
	rootId: string | null,
	path: string | null,
	preview: string | null,
) {
	const params = new URLSearchParams();
	if (rootId) params.set('root', rootId);
	if (path !== null) params.set('path', path || '/');
	if (preview) params.set('preview', preview);

	goto(`?${params.toString()}`, { replaceState: false, keepFocus: true });
}

function handleSelect(rootId: string, path: string, isDir: boolean) {
	// If directory selected, update root/path, clear preview
	if (isDir) {
		updateUrl(rootId, path, null);
	}
}

function handleNavigate(path: string) {
	if (selectedRootId) {
		updateUrl(selectedRootId, path, null);
	}
}

function handlePreviewChange(path: string | null) {
	if (selectedRootId && selectedPath) {
		updateUrl(selectedRootId, selectedPath, path);
	}
}
</script>

<div class="flex h-screen bg-gray-950 text-gray-100">
	<!-- Tree sidebar -->
	<div class="w-72 flex-shrink-0 border-r border-gray-800">
		<div class="flex h-12 items-center border-b border-gray-800 px-4">
			<h1 class="text-sm font-semibold text-gray-300">File Manager</h1>
		</div>
		<div class="h-[calc(100vh-3rem)]">
			<TreeView selectedPath={selectedKey} onSelect={handleSelect} />
		</div>
	</div>

	<!-- Content area -->
	<div class="flex-1">
		{#if selectedRootId && selectedPath != null}
			<ThumbnailGrid
				rootId={selectedRootId}
				path={selectedPath}
				previewFile={previewPath}
				onNavigate={handleNavigate}
				onPreviewChange={handlePreviewChange}
			/>
		{:else}
			<div class="flex h-full items-center justify-center">
				<p class="text-gray-600">Select a directory to browse</p>
			</div>
		{/if}
	</div>
</div>

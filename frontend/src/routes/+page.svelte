<script lang="ts">
	import TreeView from '$lib/components/TreeView.svelte';
	import ThumbnailGrid from '$lib/components/ThumbnailGrid.svelte';

	let selectedRootId = $state<string | null>(null);
	let selectedPath = $state<string | null>(null);
	let selectedKey = $state<string | null>(null);

	function handleSelect(rootId: string, path: string, isDir: boolean) {
		selectedKey = `${rootId}:${path}`;
		if (isDir) {
			selectedRootId = rootId;
			selectedPath = path;
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
		{#if selectedRootId && selectedPath}
			<ThumbnailGrid rootId={selectedRootId} path={selectedPath} />
		{:else}
			<div class="flex h-full items-center justify-center">
				<p class="text-gray-600">Select a directory to browse</p>
			</div>
		{/if}
	</div>
</div>

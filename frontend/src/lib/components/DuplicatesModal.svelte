<script lang="ts">
import { findDuplicates } from '$lib/api';
import type { DuplicateGroup } from '$lib/types';
import { formatSize } from '$lib/utils';
import { X } from '@lucide/svelte';

interface Props {
	rootId: string;
	path: string;
	onClose: () => void;
}

let { rootId, path, onClose }: Props = $props();

let loading = $state(true);
let groups = $state<DuplicateGroup[]>([]);

$effect(() => {
	loading = true;
	findDuplicates(rootId, path || '/')
		.then((r) => {
			groups = r.groups || [];
		})
		.catch(() => {
			groups = [];
		})
		.finally(() => {
			loading = false;
		});
});
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
		class="w-[600px] max-h-[80vh] overflow-y-auto rounded-lg bg-gray-900 border border-gray-700 shadow-2xl"
		data-testid="duplicates-modal"
		onclick={(e) => e.stopPropagation()}
	>
		<div class="flex items-center justify-between border-b border-gray-800 px-6 py-4">
			<h2 class="text-lg font-semibold text-gray-100">Duplicate Files</h2>
			<button class="text-gray-500 hover:text-gray-300" onclick={onClose}>
				<X size={18} />
			</button>
		</div>
		<div class="p-6">
			{#if loading}
				<p class="text-gray-500 text-center">Searching for duplicates...</p>
			{:else if groups.length === 0}
				<p class="text-gray-500 text-center">No duplicates found</p>
			{:else}
				<div class="space-y-4">
					{#each groups as group}
						<div class="rounded border border-gray-700 bg-gray-800/50 p-4">
							<div class="flex items-center gap-2 mb-2">
								<span class="text-xs text-gray-500 font-mono">{group.hashType}: {group.hash.slice(0, 16)}...</span>
								<span class="text-xs text-gray-400">{formatSize(group.size)}</span>
							</div>
							<ul class="space-y-1">
								{#each group.files as file}
									<li class="text-sm text-gray-300 flex items-center gap-2">
										<span class="text-gray-500 text-xs">{file.rootId}</span>
										<span class="break-all">{file.path}</span>
									</li>
								{/each}
							</ul>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	</div>
</div>

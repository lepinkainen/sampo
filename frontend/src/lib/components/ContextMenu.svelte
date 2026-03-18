<script lang="ts">
import {
	Scissors,
	Copy,
	ClipboardPaste,
	Trash2,
	FolderOpen,
} from '@lucide/svelte';

interface MenuItem {
	label: string;
	icon: typeof Copy;
	action: () => void;
	disabled?: boolean;
	destructive?: boolean;
}

interface Props {
	x: number;
	y: number;
	items: MenuItem[];
	onClose: () => void;
}

let { x, y, items, onClose }: Props = $props();

// Adjust position so menu doesn't overflow viewport
let menuEl: HTMLDivElement | undefined = $state();

let adjustedX = $derived.by(() => {
	if (!menuEl) return x;
	const maxX = window.innerWidth - menuEl.offsetWidth - 8;
	return Math.min(x, maxX);
});

let adjustedY = $derived.by(() => {
	if (!menuEl) return y;
	const maxY = window.innerHeight - menuEl.offsetHeight - 8;
	return Math.min(y, maxY);
});

function handleKeydown(e: KeyboardEvent) {
	if (e.key === 'Escape') onClose();
}
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="fixed inset-0 z-40" onclick={onClose}>
	<div
		bind:this={menuEl}
		class="fixed z-50 min-w-[160px] rounded-lg border border-gray-700 bg-gray-900 py-1 shadow-xl"
		style="left: {adjustedX}px; top: {adjustedY}px"
		onclick={(e) => e.stopPropagation()}
	>
		{#each items as item}
			<button
				class="flex w-full items-center gap-2 px-3 py-1.5 text-sm transition-colors
				{item.disabled
					? 'text-gray-600 cursor-not-allowed'
					: item.destructive
						? 'text-red-400 hover:bg-red-900/30'
						: 'text-gray-300 hover:bg-gray-800'}"
				disabled={item.disabled}
				onclick={() => {
					item.action();
					onClose();
				}}
			>
				<item.icon size={14} />
				{item.label}
			</button>
		{/each}
	</div>
</div>

<style>
	/* Re-export icon components so they can be referenced from parent */
</style>

<script lang="ts">
import { Pencil } from '@lucide/svelte';

interface Props {
	currentName: string;
	onConfirm: (newName: string) => void;
	onCancel: () => void;
}

let { currentName, onConfirm, onCancel }: Props = $props();

let newName = $state('');
let lastCurrentName = $state('');
let inputEl: HTMLInputElement | undefined = $state();

let isDisabled = $derived(newName.trim() === '' || newName === currentName);

$effect(() => {
	if (currentName !== lastCurrentName) {
		newName = currentName;
		lastCurrentName = currentName;
	}
});

$effect(() => {
	if (inputEl) {
		inputEl.focus();
		// Select name without extension for files
		const dotIndex = currentName.lastIndexOf('.');
		if (dotIndex > 0) {
			inputEl.setSelectionRange(0, dotIndex);
		} else {
			inputEl.select();
		}
	}
});

function handleKeydown(e: KeyboardEvent) {
	if (e.key === 'Escape') {
		e.stopPropagation();
		onCancel();
	}
	if (e.key === 'Enter' && !isDisabled) {
		e.stopPropagation();
		onConfirm(newName.trim());
	}
}
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
	onclick={onCancel}
>
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="mx-4 w-full max-w-md rounded-xl bg-gray-900 p-6 shadow-2xl border border-gray-700"
		onclick={(e) => e.stopPropagation()}
		onkeydown={handleKeydown}
	>
		<div class="flex items-center gap-3 mb-4">
			<div class="flex h-10 w-10 items-center justify-center rounded-full bg-blue-900/50">
				<Pencil size={20} class="text-blue-400" />
			</div>
			<h2 class="text-lg font-semibold text-gray-100">Rename</h2>
		</div>

		<input
			bind:this={inputEl}
			bind:value={newName}
			class="mb-4 w-full rounded-lg bg-gray-950 border border-gray-700 px-3 py-2 text-sm text-gray-200 focus:outline-none focus:border-blue-500"
			type="text"
		/>

		<div class="flex justify-end gap-3">
			<button
				class="rounded-lg px-4 py-2 text-sm font-medium text-gray-300 hover:bg-gray-800 transition-colors"
				onclick={onCancel}
			>
				Cancel
			</button>
			<button
				class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
				disabled={isDisabled}
				onclick={() => onConfirm(newName.trim())}
			>
				Rename
			</button>
		</div>
	</div>
</div>

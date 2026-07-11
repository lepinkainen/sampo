<script lang="ts">
import { tick } from 'svelte';
import { Pencil } from '@lucide/svelte';

interface Props {
	currentName: string;
	onConfirm: (newName: string) => void;
	onCancel: () => void;
}

let { currentName, onConfirm, onCancel }: Props = $props();

// Initialize from the prop so the input already has its value on first
// render — seeding it from an effect lands after the focus/select effect,
// which collapses the name-without-extension selection to the end. The
// initial-value capture is deliberate; the $effect below syncs later
// currentName changes.
// svelte-ignore state_referenced_locally
let newName = $state(currentName);
// svelte-ignore state_referenced_locally
let lastCurrentName = $state(currentName);
let inputEl: HTMLInputElement | undefined = $state();

let isDisabled = $derived(newName.trim() === '' || newName === currentName);

$effect(() => {
	if (currentName !== lastCurrentName) {
		newName = currentName;
		lastCurrentName = currentName;
	}
});

$effect(() => {
	if (!inputEl) {
		return;
	}
	const el = inputEl;
	el.focus();
	// Select name without extension for files. Deferred past the current
	// flush: value writes landing after this effect reset the selection to
	// the end of the input.
	void tick().then(() => {
		const dotIndex = el.value.lastIndexOf('.');
		if (dotIndex > 0) {
			el.setSelectionRange(0, dotIndex);
		} else {
			el.select();
		}
	});
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
		class="mx-4 w-full max-w-md rounded-xl bg-surface p-6 shadow-2xl border border-muted"
		onclick={(e) => e.stopPropagation()}
		onkeydown={handleKeydown}
	>
		<div class="flex items-center gap-3 mb-4">
			<div class="flex h-10 w-10 items-center justify-center rounded-full bg-accent-deep/50">
				<Pencil size={20} class="text-accent-soft" />
			</div>
			<h2 class="text-lg font-semibold text-bright">Rename</h2>
		</div>

		<input
			bind:this={inputEl}
			bind:value={newName}
			class="mb-4 w-full rounded-lg bg-canvas border border-muted px-3 py-2 text-sm text-body focus:outline-none focus:border-accent-hover"
			type="text"
		/>

		<div class="flex justify-end gap-3">
			<button
				class="rounded-lg px-4 py-2 text-sm font-medium text-body hover:bg-raised transition-colors"
				onclick={onCancel}
			>
				Cancel
			</button>
			<button
				class="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-on-accent hover:bg-accent-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
				disabled={isDisabled}
				onclick={() => onConfirm(newName.trim())}
			>
				Rename
			</button>
		</div>
	</div>
</div>

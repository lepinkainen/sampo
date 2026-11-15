<script lang="ts">
import { tick } from 'svelte';
import { Pencil } from '@lucide/svelte';
import Modal from './Modal.svelte';

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

const titleId = $props.id();

// Enter submits through the form's native implicit submission, which
// browsers suppress while an IME composition is confirming a candidate —
// a custom keydown handler would rename mid-composition.
function handleSubmit(e: SubmitEvent) {
	e.preventDefault();
	if (!isDisabled) onConfirm(newName.trim());
}
</script>

<Modal
	labelledby={titleId}
	class="w-[calc(100%-2rem)] max-w-md p-6"
	testid="rename-dialog"
	onClose={onCancel}
>
	<form onsubmit={handleSubmit}>
		<div class="flex items-center gap-3 mb-4">
			<div class="flex h-10 w-10 items-center justify-center rounded-full bg-accent-deep/50">
				<Pencil size={20} class="text-accent-soft" aria-hidden="true" />
			</div>
			<h2 id={titleId} class="text-lg font-semibold text-bright">Rename</h2>
		</div>

		<input
			bind:this={inputEl}
			bind:value={newName}
			aria-label="New name"
			autocomplete="off"
			spellcheck="false"
			class="mb-4 w-full rounded-lg bg-canvas border border-muted px-3 py-2 text-sm text-body focus:outline-none focus:border-accent-hover focus-visible:ring-1 focus-visible:ring-accent-hover"
			type="text"
		/>

		<div class="flex justify-end gap-3">
			<button
				type="button"
				class="rounded-lg px-4 py-2 text-sm font-medium text-body hover:bg-raised transition-colors"
				onclick={onCancel}
			>
				Cancel
			</button>
			<button
				type="submit"
				class="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-on-accent hover:bg-accent-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
				disabled={isDisabled}
			>
				Rename
			</button>
		</div>
	</form>
</Modal>

<script lang="ts">
import type { Snippet } from 'svelte';

interface Props {
	/** id of the heading element that names the dialog. */
	labelledby: string;
	class?: string;
	style?: string;
	testid?: string;
	/** Called when the dialog is dismissed (Esc, back gesture, backdrop click). */
	onClose: () => void;
	children: Snippet;
}

let {
	labelledby,
	class: klass = '',
	style,
	testid,
	onClose,
	children,
}: Props = $props();

let dialogEl: HTMLDialogElement | undefined = $state();
// Set while the component tears itself down, so the close event fired by
// our own close() isn't reported to the parent as a user dismissal.
let tearingDown = false;
let pressedOutside = false;

// Light dismiss is native where `closedby` is supported; elsewhere fall back
// to a pointer-position test (the <dialog> is also the event target for its
// own padding, so a target check would close on those clicks).
const nativeLightDismiss =
	typeof HTMLDialogElement !== 'undefined' &&
	'closedBy' in HTMLDialogElement.prototype;

function isOutside(e: PointerEvent): boolean {
	if (!dialogEl) return false;
	const r = dialogEl.getBoundingClientRect();
	return (
		e.clientX < r.left ||
		e.clientX > r.right ||
		e.clientY < r.top ||
		e.clientY > r.bottom
	);
}

$effect(() => {
	const el = dialogEl;
	if (!el) return;
	if (!el.open) {
		if (typeof el.showModal === 'function') {
			el.showModal();
		} else {
			el.setAttribute('open', '');
		}
	}
	// Destructive confirmations mark their safe action; everything else gets
	// the browser's default (first focusable element).
	el.querySelector<HTMLElement>('[data-autofocus]')?.focus();
	return () => {
		tearingDown = true;
		// Close before the node is removed so the browser returns focus to
		// the element that had it when the dialog opened.
		if (el.open && typeof el.close === 'function') el.close();
	};
});
</script>

<dialog
	bind:this={dialogEl}
	aria-labelledby={labelledby}
	data-testid={testid}
	class="modal m-auto border border-muted bg-surface text-body shadow-2xl rounded-xl {klass}"
	{style}
	{...{ closedby: 'any' }}
	onclose={() => {
		if (!tearingDown) onClose();
	}}
	onpointerdown={(e) => {
		pressedOutside = !nativeLightDismiss && isOutside(e);
	}}
	onpointerup={(e) => {
		// Require press and release outside, so a text-selection drag that
		// ends on the backdrop does not dismiss.
		if (pressedOutside && isOutside(e)) dialogEl?.close();
		pressedOutside = false;
	}}
>
	{@render children()}
</dialog>

<style>
	.modal:modal {
		overscroll-behavior: contain;
	}
	.modal::backdrop {
		background: rgb(0 0 0 / 0.6);
		overflow: hidden;
		overscroll-behavior: contain;
	}
</style>

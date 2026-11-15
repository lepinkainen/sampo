<script lang="ts">
import { onMount } from 'svelte';
import type { Copy } from '@lucide/svelte';

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

// Element that had focus when the menu opened; focus returns there on close.
let opener: HTMLElement | null = null;

function enabledItems(): HTMLButtonElement[] {
	if (!menuEl) return [];
	return [
		...menuEl.querySelectorAll<HTMLButtonElement>(
			'[role="menuitem"]:not(:disabled)',
		),
	];
}

function restoreFocus() {
	const target = opener;
	opener = null;
	if (target?.isConnected) target.focus({ preventScroll: true });
}

function close() {
	restoreFocus();
	onClose();
}

onMount(() => {
	opener =
		document.activeElement instanceof HTMLElement
			? document.activeElement
			: null;
	(enabledItems()[0] ?? menuEl)?.focus({ preventScroll: true });

	return () => {
		// Closed by the parent rather than through close(): only pull focus
		// back if nothing else (e.g. a dialog opened meanwhile) has taken it.
		const active = document.activeElement;
		if (!active || active === document.body || menuEl?.contains(active)) {
			restoreFocus();
		}
	};
});

function handleMenuKeydown(e: KeyboardEvent) {
	// The menu is modal: keep its keys away from the grid/tree shortcuts.
	e.stopPropagation();

	if (e.key === 'Escape' || e.key === 'Tab') {
		e.preventDefault();
		close();
		return;
	}

	const enabled = enabledItems();
	if (enabled.length === 0) return;
	const current = enabled.indexOf(document.activeElement as HTMLButtonElement);
	let next: number;
	switch (e.key) {
		case 'ArrowDown':
			next = (current + 1) % enabled.length;
			break;
		case 'ArrowUp':
			next = current <= 0 ? enabled.length - 1 : current - 1;
			break;
		case 'Home':
			next = 0;
			break;
		case 'End':
			next = enabled.length - 1;
			break;
		default:
			return;
	}
	e.preventDefault();
	enabled[next].focus();
}

// Fallback for when focus has left the menu (e.g. after a click on the page
// chrome); keys pressed inside the menu never get here.
function handleWindowKeydown(e: KeyboardEvent) {
	if (e.key === 'Escape') {
		e.preventDefault();
		close();
	}
}
</script>

<svelte:window onkeydown={handleWindowKeydown} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="fixed inset-0 z-40" onclick={close}>
	<div
		bind:this={menuEl}
		class="fixed z-50 min-w-[160px] rounded-lg border border-muted bg-surface py-1 shadow-xl outline-none"
		style="left: {adjustedX}px; top: {adjustedY}px"
		role="menu"
		aria-label="File actions"
		tabindex="-1"
		onclick={(e) => e.stopPropagation()}
		onkeydown={handleMenuKeydown}
	>
		{#each items as item}
			<button
				type="button"
				role="menuitem"
				tabindex="-1"
				class="flex w-full items-center gap-2 px-3 py-1.5 text-sm transition-colors outline-none
				{item.disabled
					? 'text-ghost cursor-not-allowed'
					: item.destructive
						? 'text-danger-soft hover:bg-danger-deep/30 focus-visible:bg-danger-deep/30'
						: 'text-body hover:bg-raised focus-visible:bg-raised'}"
				disabled={item.disabled}
				onclick={() => {
					// Hand focus back first so a dialog opened by the action
					// sees the original element as its opener.
					restoreFocus();
					item.action();
					onClose();
				}}
			>
				<item.icon size={14} aria-hidden="true" />
				{item.label}
			</button>
		{/each}
	</div>
</div>

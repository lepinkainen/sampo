<script lang="ts">
import { ImageOff, Trash2 } from '@lucide/svelte';

interface Props {
	title: string;
	items: Array<{ name: string; thumbUrl?: string }>;
	confirmLabel?: string;
	onConfirm: () => void;
	onCancel: () => void;
}

let {
	title,
	items,
	confirmLabel = 'Delete',
	onConfirm,
	onCancel,
}: Props = $props();

let thumbErrors = $state<Record<number, boolean>>({});

function handleKeydown(e: KeyboardEvent) {
	if (e.key === 'Escape') onCancel();
	if (e.key === 'Enter') onConfirm();
}
</script>

<svelte:window onkeydown={handleKeydown} />

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
	>
		<div class="flex items-center gap-3 mb-4">
			<div class="flex h-10 w-10 items-center justify-center rounded-full bg-danger-deep/50">
				<Trash2 size={20} class="text-danger-soft" />
			</div>
			<h2 class="text-lg font-semibold text-bright">{title}</h2>
		</div>

		{#if items.length > 0}
			<div class="mb-4 max-h-40 overflow-y-auto rounded-lg bg-canvas p-3">
				{#each items as item, i}
					<div class="flex items-center gap-2 py-0.5">
						{#if item.thumbUrl && !thumbErrors[i]}
							<img
								src={item.thumbUrl}
								alt=""
								loading="lazy"
								class="h-9 w-9 shrink-0 rounded-md border border-raised object-cover"
								onerror={() => {
									thumbErrors[i] = true;
								}}
							/>
						{:else}
							<div
								class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-raised bg-raised"
							>
								<ImageOff size={16} class="text-ghost" />
							</div>
						{/if}
						<p class="truncate text-sm text-dim">{item.name}</p>
					</div>
				{/each}
			</div>
		{/if}

		<div class="flex justify-end gap-3">
			<button
				class="rounded-lg px-4 py-2 text-sm font-medium text-body hover:bg-raised transition-colors"
				onclick={onCancel}
			>
				Cancel
			</button>
			<button
				class="rounded-lg bg-danger px-4 py-2 text-sm font-medium text-white hover:bg-danger-strong transition-colors"
				onclick={onConfirm}
			>
				{confirmLabel}
			</button>
		</div>
	</div>
</div>

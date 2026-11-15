<script lang="ts">
import { ImageOff, Trash2 } from '@lucide/svelte';
import Modal from './Modal.svelte';

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

const titleId = $props.id();
</script>

<Modal
	labelledby={titleId}
	class="w-[calc(100%-2rem)] max-w-md p-6"
	testid="confirm-dialog"
	onClose={onCancel}
>
		<div class="flex items-center gap-3 mb-4">
			<div class="flex h-10 w-10 items-center justify-center rounded-full bg-danger-deep/50">
				<Trash2 size={20} class="text-danger-soft" aria-hidden="true" />
			</div>
			<h2 id={titleId} class="text-lg font-semibold text-bright">{title}</h2>
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
			<!-- Initial focus goes to the safe action, so a stray Enter cancels. -->
			<button
				type="button"
				data-autofocus
				class="rounded-lg px-4 py-2 text-sm font-medium text-body hover:bg-raised transition-colors"
				onclick={onCancel}
			>
				Cancel
			</button>
			<button
				type="button"
				class="rounded-lg bg-danger px-4 py-2 text-sm font-medium text-white hover:bg-danger-strong transition-colors"
				onclick={onConfirm}
			>
				{confirmLabel}
			</button>
		</div>
</Modal>

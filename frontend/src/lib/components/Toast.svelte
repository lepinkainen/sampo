<script lang="ts">
import { CheckCircle, XCircle, X } from '@lucide/svelte';

interface Toast {
	id: number;
	message: string;
	type: 'success' | 'error';
}

let toasts = $state<Toast[]>([]);
let nextId = 0;

export function show(message: string, type: 'success' | 'error' = 'success') {
	const id = nextId++;
	toasts = [...toasts, { id, message, type }];
	setTimeout(() => {
		toasts = toasts.filter((t) => t.id !== id);
	}, 3000);
}

function dismiss(id: number) {
	toasts = toasts.filter((t) => t.id !== id);
}
</script>

<div class="pointer-events-none fixed bottom-4 right-4 z-50 flex flex-col gap-2">
	{#each toasts as toast (toast.id)}
		<div
			class="pointer-events-auto flex items-center gap-2 rounded-lg px-4 py-3 text-sm font-medium shadow-lg transition-all
			{toast.type === 'success' ? 'bg-green-900/90 text-green-100' : 'bg-red-900/90 text-red-100'}"
		>
			{#if toast.type === 'success'}
				<CheckCircle size={16} />
			{:else}
				<XCircle size={16} />
			{/if}
			<span>{toast.message}</span>
			<button
				class="ml-2 opacity-60 hover:opacity-100"
				onclick={() => dismiss(toast.id)}
			>
				<X size={14} />
			</button>
		</div>
	{/each}
</div>

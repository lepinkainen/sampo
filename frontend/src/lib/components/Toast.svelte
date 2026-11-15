<script lang="ts">
import { CheckCircle, XCircle, X } from '@lucide/svelte';
import { getToasts, dismissToast } from '$lib/toast.svelte';

const toasts = $derived(getToasts());
</script>

<div
	class="pointer-events-none fixed bottom-4 right-4 z-50 flex flex-col gap-2"
	role="region"
	aria-label="Notifications"
	aria-live="polite"
>
	{#each toasts as toast (toast.id)}
		<div
			class="pointer-events-auto flex items-center gap-2 rounded-lg px-4 py-3 text-sm font-medium shadow-lg transition-all
			{toast.type === 'success' ? 'bg-ok-deep/90 text-ok-faint' : 'bg-danger-deep/90 text-danger-faint'}"
			role={toast.type === 'error' ? 'alert' : undefined}
		>
			{#if toast.type === 'success'}
				<CheckCircle size={16} aria-hidden="true" />
			{:else}
				<XCircle size={16} aria-hidden="true" />
			{/if}
			<span>{toast.message}</span>
			<button
				type="button"
				class="ml-2 opacity-60 hover:opacity-100"
				aria-label="Dismiss notification"
				onclick={() => dismissToast(toast.id)}
			>
				<X size={14} aria-hidden="true" />
			</button>
		</div>
	{/each}
</div>

import type { ItemResult } from '$lib/api';

export interface Toast {
	id: number;
	message: string;
	type: 'success' | 'error';
}

let nextId = 0;
let toasts = $state<Toast[]>([]);

export function getToasts(): Toast[] {
	return toasts;
}

export function showToast(
	message: string,
	type: 'success' | 'error' = 'success',
): void {
	const id = nextId++;
	toasts = [...toasts, { id, message, type }];
	setTimeout(() => {
		toasts = toasts.filter((t) => t.id !== id);
	}, 3000);
}

export function dismissToast(id: number): void {
	toasts = toasts.filter((t) => t.id !== id);
}

export function summarizeItemErrors(results: ItemResult[]): string | null {
	const failures = results.filter((r) => r.error);
	if (failures.length === 0) return null;
	if (failures.length === 1) return failures[0].error ?? null;
	return `${failures.length} of ${results.length} item(s) failed: ${failures[0].error}`;
}

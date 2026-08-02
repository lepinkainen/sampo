// Tiny localStorage-backed UI preference helpers. Mirrors the try/catch
// pattern in theme.svelte.ts so private mode / disabled storage degrades to
// session-only defaults instead of throwing.

export function readPref<T extends string>(
	key: string,
	allowed: readonly T[],
	fallback: T,
): T {
	try {
		const v = localStorage.getItem(key);
		return allowed.includes(v as T) ? (v as T) : fallback;
	} catch {
		return fallback;
	}
}

export function writePref(key: string, value: string): void {
	try {
		localStorage.setItem(key, value);
	} catch {
		// Storage disabled: preference still applies for the current session.
	}
}

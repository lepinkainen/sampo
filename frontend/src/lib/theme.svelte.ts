export const THEMES = ['dark', 'swiss', 'terminal'] as const;
export type Theme = (typeof THEMES)[number];

const STORAGE_KEY = 'sampo-theme';

function initialTheme(): Theme {
	// app.html sets data-theme from localStorage before first paint; trust
	// the attribute so the store and DOM agree even if storage is disabled.
	const attr = document.documentElement.dataset.theme;
	return THEMES.includes(attr as Theme) ? (attr as Theme) : 'dark';
}

export const theme = $state<{ current: Theme }>({ current: initialTheme() });

export function setTheme(next: Theme) {
	theme.current = next;
	document.documentElement.dataset.theme = next;
	try {
		localStorage.setItem(STORAGE_KEY, next);
	} catch {
		// Private mode / storage disabled: theme still applies for the session.
	}
}

export function cycleTheme() {
	const idx = THEMES.indexOf(theme.current);
	setTheme(THEMES[(idx + 1) % THEMES.length]);
}

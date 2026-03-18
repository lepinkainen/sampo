import type { FileEntry } from './types';

export function createSelection() {
	let selected = $state<Set<string>>(new Set());
	let lastClicked = $state<string | null>(null);

	function clear() {
		selected = new Set();
		lastClicked = null;
	}

	function selectOne(entry: FileEntry) {
		selected = new Set([entry.path]);
		lastClicked = entry.path;
	}

	function toggle(entry: FileEntry) {
		const next = new Set(selected);
		if (next.has(entry.path)) {
			next.delete(entry.path);
		} else {
			next.add(entry.path);
		}
		selected = next;
		lastClicked = entry.path;
	}

	function selectRange(entries: FileEntry[], entry: FileEntry) {
		if (!lastClicked) {
			selectOne(entry);
			return;
		}
		const paths = entries.map((e) => e.path);
		const from = paths.indexOf(lastClicked);
		const to = paths.indexOf(entry.path);
		if (from === -1 || to === -1) {
			selectOne(entry);
			return;
		}
		const start = Math.min(from, to);
		const end = Math.max(from, to);
		const next = new Set(selected);
		for (let i = start; i <= end; i++) {
			next.add(paths[i]);
		}
		selected = next;
	}

	function selectAll(entries: FileEntry[]) {
		selected = new Set(entries.map((e) => e.path));
	}

	function has(path: string): boolean {
		return selected.has(path);
	}

	return {
		get selected() {
			return selected;
		},
		get size() {
			return selected.size;
		},
		get paths() {
			return [...selected];
		},
		clear,
		selectOne,
		toggle,
		selectRange,
		selectAll,
		has,
	};
}

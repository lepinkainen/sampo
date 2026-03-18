export type ClipboardMode = 'copy' | 'cut';

export interface ClipboardItem {
	rootId: string;
	path: string;
}

export function createClipboard() {
	let items = $state<ClipboardItem[]>([]);
	let mode = $state<ClipboardMode>('copy');

	function copy(rootId: string, paths: string[]) {
		items = paths.map((p) => ({ rootId, path: p }));
		mode = 'copy';
	}

	function cut(rootId: string, paths: string[]) {
		items = paths.map((p) => ({ rootId, path: p }));
		mode = 'cut';
	}

	function clear() {
		items = [];
	}

	function isCut(rootId: string, path: string): boolean {
		return (
			mode === 'cut' &&
			items.some((i) => i.rootId === rootId && i.path === path)
		);
	}

	return {
		get items() {
			return items;
		},
		get mode() {
			return mode;
		},
		get hasItems() {
			return items.length > 0;
		},
		copy,
		cut,
		clear,
		isCut,
	};
}

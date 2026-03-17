import type { FileEntry, Root } from './types';

const BASE = '';

function encodePath(path: string): string {
	return path.split('/').map(encodeURIComponent).join('/');
}

export async function fetchRoots(): Promise<Root[]> {
	const res = await fetch(`${BASE}/api/roots`);
	if (!res.ok) throw new Error(`Failed to fetch roots: ${res.statusText}`);
	return res.json();
}

export async function fetchDirectory(
	rootId: string,
	path: string,
): Promise<FileEntry[]> {
	const res = await fetch(`${BASE}/api/tree/${rootId}/${encodePath(path)}`);
	if (!res.ok) throw new Error(`Failed to fetch directory: ${res.statusText}`);
	return res.json();
}

export function thumbnailUrl(rootId: string, path: string): string {
	return `${BASE}/api/thumb/${rootId}/${encodePath(path)}`;
}

export function fileUrl(rootId: string, path: string): string {
	return `${BASE}/api/file/${rootId}/${encodePath(path)}`;
}

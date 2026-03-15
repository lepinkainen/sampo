import type { Root, FileEntry } from './types';

const BASE = '';

export async function fetchRoots(): Promise<Root[]> {
	const res = await fetch(`${BASE}/api/roots`);
	if (!res.ok) throw new Error(`Failed to fetch roots: ${res.statusText}`);
	return res.json();
}

export async function fetchDirectory(rootId: string, path: string): Promise<FileEntry[]> {
	const encodedPath = path.split('/').map(encodeURIComponent).join('/');
	const res = await fetch(`${BASE}/api/tree/${rootId}/${encodedPath}`);
	if (!res.ok) throw new Error(`Failed to fetch directory: ${res.statusText}`);
	return res.json();
}

export function thumbnailUrl(rootId: string, path: string): string {
	const encodedPath = path.split('/').map(encodeURIComponent).join('/');
	return `${BASE}/api/thumb/${rootId}/${encodedPath}`;
}

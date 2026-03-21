import type { DuplicatesResponse, FileEntry, Root, TagScore } from './types';

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
	options?: { filter?: string; tag?: string },
): Promise<FileEntry[]> {
	let url = `${BASE}/api/tree/${rootId}/${encodePath(path)}`;
	const params = new URLSearchParams();
	if (options?.filter) params.set('filter', options.filter);
	if (options?.tag) params.set('tag', options.tag);
	const qs = params.toString();
	if (qs) url += `?${qs}`;
	const res = await fetch(url);
	if (!res.ok) throw new Error(`Failed to fetch directory: ${res.statusText}`);
	return res.json();
}

export function thumbnailUrl(rootId: string, path: string): string {
	return `${BASE}/api/thumb/${rootId}/${encodePath(path)}`;
}

export function fileUrl(rootId: string, path: string): string {
	return `${BASE}/api/file/${rootId}/${encodePath(path)}`;
}

export interface BrowseAnalysisStatus {
	pending: number;
	queued: number;
	active: number;
	running: boolean;
}

export interface AnalysisSettings {
	autoBrowseEnabled: boolean;
	browseStatus: BrowseAnalysisStatus;
}

export async function getAnalysisSettings(): Promise<AnalysisSettings> {
	const res = await fetch(`${BASE}/api/analysis/settings`);
	if (!res.ok)
		throw new Error(`Failed to fetch analysis settings: ${res.statusText}`);
	return res.json();
}

export async function setAnalysisSettings(
	autoBrowseEnabled: boolean,
): Promise<AnalysisSettings> {
	const res = await fetch(`${BASE}/api/analysis/settings`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ autoBrowseEnabled }),
	});
	if (!res.ok)
		throw new Error(`Failed to update analysis settings: ${res.statusText}`);
	return res.json();
}

export interface BulkRequest {
	items: { srcRoot: string; srcPath: string }[];
	dstRoot: string;
	dstPath: string;
}

export interface ItemResult {
	srcRoot: string;
	srcPath: string;
	dstPath?: string;
	error?: string;
}

export async function deleteFiles(
	rootId: string,
	paths: string[],
	recursive = false,
): Promise<void> {
	const results = await Promise.allSettled(
		paths.map(async (p) => {
			const url = `${BASE}/api/files/${rootId}/${encodePath(p)}${recursive ? '?recursive=true' : ''}`;
			const res = await fetch(url, { method: 'DELETE' });
			if (!res.ok) {
				const text = await res.text();
				throw new Error(text || res.statusText);
			}
		}),
	);
	const errors = results.filter(
		(r): r is PromiseRejectedResult => r.status === 'rejected',
	);
	if (errors.length > 0) {
		throw new Error(
			`Failed to delete ${errors.length} item(s): ${errors.map((e) => e.reason).join(', ')}`,
		);
	}
}

export async function moveFiles(req: BulkRequest): Promise<ItemResult[]> {
	const res = await fetch(`${BASE}/api/files/move`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(req),
	});
	if (!res.ok && res.status !== 207) {
		throw new Error(`Move failed: ${res.statusText}`);
	}
	return res.json();
}

export async function renameFile(
	rootId: string,
	path: string,
	newName: string,
): Promise<{ newName: string }> {
	const res = await fetch(`${BASE}/api/files/rename`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ rootId, path, newName }),
	});
	if (!res.ok) {
		const text = await res.text();
		throw new Error(text || res.statusText);
	}
	return res.json();
}

export async function copyFiles(req: BulkRequest): Promise<ItemResult[]> {
	const res = await fetch(`${BASE}/api/files/copy`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(req),
	});
	if (!res.ok && res.status !== 207) {
		throw new Error(`Copy failed: ${res.statusText}`);
	}
	return res.json();
}

// Detection API

export interface DetectionResult {
	rootId: string;
	relPath: string;
	hasPerson: boolean;
	confidence: number;
	modelVer: string;
	scannedAt: string;
}

export interface ScanStatus {
	running: boolean;
	rootId?: string;
	path?: string;
	total: number;
	completed: number;
	errors: number;
}

export async function getDetection(
	rootId: string,
	path: string,
): Promise<DetectionResult> {
	const res = await fetch(`${BASE}/api/detect/${rootId}/${encodePath(path)}`);
	if (!res.ok) throw new Error(`Detection failed: ${res.statusText}`);
	return res.json();
}

export async function startScan(
	rootId: string,
	path: string,
): Promise<ScanStatus> {
	const res = await fetch(`${BASE}/api/detect/scan`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ rootId, path }),
	});
	if (!res.ok) throw new Error(`Scan failed: ${res.statusText}`);
	return res.json();
}

export async function getScanStatus(): Promise<ScanStatus> {
	const res = await fetch(`${BASE}/api/detect/status`);
	if (!res.ok) throw new Error(`Status failed: ${res.statusText}`);
	return res.json();
}

// Classification API

export interface ClassificationResult {
	rootId: string;
	relPath: string;
	tags: TagScore[];
	modelVer: string;
	scannedAt: string;
}

export async function getClassification(
	rootId: string,
	path: string,
): Promise<ClassificationResult> {
	const res = await fetch(`${BASE}/api/classify/${rootId}/${encodePath(path)}`);
	if (!res.ok) throw new Error(`Classification failed: ${res.statusText}`);
	return res.json();
}

export async function startClassifyScan(
	rootId: string,
	path: string,
): Promise<ScanStatus> {
	const res = await fetch(`${BASE}/api/classify/scan`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ rootId, path }),
	});
	if (!res.ok) throw new Error(`Classification scan failed: ${res.statusText}`);
	return res.json();
}

export async function getClassifyScanStatus(): Promise<ScanStatus> {
	const res = await fetch(`${BASE}/api/classify/status`);
	if (!res.ok) throw new Error(`Status failed: ${res.statusText}`);
	return res.json();
}

// Search API

export async function searchFiles(
	rootId: string,
	query: string,
	path?: string,
): Promise<FileEntry[]> {
	const params = new URLSearchParams({ q: query });
	if (path) params.set('path', path);
	const res = await fetch(`${BASE}/api/search/${rootId}?${params}`);
	if (!res.ok) throw new Error(`Search failed: ${res.statusText}`);
	return res.json();
}

// Disk Usage API

export interface DiskUsage {
	totalSize: number;
	fileCount: number;
	dirCount: number;
}

export async function getDiskUsage(
	rootId: string,
	path: string,
): Promise<DiskUsage> {
	const res = await fetch(`${BASE}/api/usage/${rootId}/${encodePath(path)}`);
	if (!res.ok) throw new Error(`Usage failed: ${res.statusText}`);
	return res.json();
}

// Duplicates API

export async function findDuplicates(
	rootId: string,
	path: string,
): Promise<DuplicatesResponse> {
	const res = await fetch(
		`${BASE}/api/duplicates/${rootId}/${encodePath(path)}`,
	);
	if (!res.ok) throw new Error(`Duplicates failed: ${res.statusText}`);
	return res.json();
}

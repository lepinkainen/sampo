import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	fetchDirectory,
	getCachedDirectory,
	invalidateDirectoryCache,
} from './api';
import type { FileEntry } from './types';

function entry(name: string): FileEntry {
	return {
		name,
		path: name,
		isDir: false,
		isZip: false,
		size: 1,
		modTime: '2026-01-01T00:00:00Z',
		mediaType: 'other',
		hasThumb: false,
	};
}

function jsonResponse(body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status: 200,
		headers: { 'Content-Type': 'application/json' },
	});
}

describe('directory API cache', () => {
	let pending: Array<(response: Response) => void>;
	let fetchMock: ReturnType<typeof vi.fn>;

	beforeEach(() => {
		pending = [];
		fetchMock = vi.fn(
			() =>
				new Promise<Response>((resolve) => {
					pending.push(resolve);
				}),
		);
		vi.stubGlobal('fetch', fetchMock);
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('collapses concurrent directory fetches for the same key', async () => {
		const root = 'root-api-collapse';
		const first = fetchDirectory(root, '/');
		const second = fetchDirectory(root, '/');

		expect(fetchMock).toHaveBeenCalledTimes(1);

		pending[0](jsonResponse([entry('one')]));

		await expect(first).resolves.toMatchObject([{ name: 'one' }]);
		await expect(second).resolves.toMatchObject([{ name: 'one' }]);
		expect(getCachedDirectory(root, '/')).toMatchObject([{ name: 'one' }]);
	});

	it('does not cache a stale in-flight response after invalidation', async () => {
		const root = 'root-api-invalidate-inflight';
		const path = '/photos';

		const stale = fetchDirectory(root, path);
		expect(fetchMock).toHaveBeenCalledTimes(1);

		invalidateDirectoryCache(root, path);
		const fresh = fetchDirectory(root, path);
		expect(fetchMock).toHaveBeenCalledTimes(2);

		pending[0](jsonResponse([entry('stale')]));
		await expect(stale).resolves.toMatchObject([{ name: 'stale' }]);
		expect(getCachedDirectory(root, path)).toBeNull();

		const alsoFresh = fetchDirectory(root, path);
		expect(fetchMock).toHaveBeenCalledTimes(2);

		pending[1](jsonResponse([entry('fresh')]));
		await expect(fresh).resolves.toMatchObject([{ name: 'fresh' }]);
		await expect(alsoFresh).resolves.toMatchObject([{ name: 'fresh' }]);
		expect(getCachedDirectory(root, path)).toMatchObject([{ name: 'fresh' }]);
	});
});

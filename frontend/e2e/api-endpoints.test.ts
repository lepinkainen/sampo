import { expect, test } from '@playwright/test';

test.describe('API endpoints', () => {
	test('health endpoint returns ok', async ({ request }) => {
		const response = await request.get('/health');
		expect(response.ok()).toBe(true);
	});

	test('whoami endpoint returns version info', async ({ request }) => {
		const response = await request.get('/whoami');
		expect(response.ok()).toBe(true);
		const body = await response.json();
		expect(body).toHaveProperty('version');
	});

	test('roots endpoint returns configured roots', async ({ request }) => {
		const response = await request.get('/api/roots');
		expect(response.ok()).toBe(true);
		const roots = await response.json();
		expect(Array.isArray(roots)).toBe(true);
		expect(roots.length).toBeGreaterThan(0);
		expect(roots[0]).toHaveProperty('id');
		expect(roots[0]).toHaveProperty('name');
	});

	test('tree endpoint returns directory listing', async ({ request }) => {
		// First get roots to find a valid root ID
		const rootsResponse = await request.get('/api/roots');
		const roots = await rootsResponse.json();
		const rootId = roots[0].id;

		const response = await request.get(`/api/tree/${rootId}/`);
		expect(response.ok()).toBe(true);
		const entries = await response.json();
		expect(Array.isArray(entries)).toBe(true);
		expect(entries.length).toBeGreaterThan(0);
		expect(entries[0]).toHaveProperty('name');
		expect(entries[0]).toHaveProperty('isDir');
	});
});

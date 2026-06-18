import { expect, test } from '@playwright/test';

const legacyTag = 'legacy-ui-tag';
const freshTag = 'fresh-ui-tag';

function imageEntry(tags: { label: string; score: number }[]) {
	return {
		name: 'tagged.jpg',
		path: 'images/tagged.jpg',
		isDir: false,
		isZip: false,
		size: 1234,
		modTime: '2026-01-01T00:00:00Z',
		mediaType: 'image',
		hasThumb: false,
		tags,
	};
}

test.describe('Re-analyze action', () => {
	test('replaces existing tags instead of appending to them', async ({
		page,
	}) => {
		let currentTags = [{ label: legacyTag, score: 0.9 }];
		let analyzePayload: unknown = null;
		let treeRequests = 0;

		await page.route('**/api/**', async (route, request) => {
			const url = new URL(request.url());

			if (url.pathname === '/api/roots') {
				await route.fulfill({
					json: [{ id: 'root-0', name: 'Sample' }],
				});
				return;
			}

			if (url.pathname === '/api/analysis/settings') {
				await route.fulfill({
					json: {
						autoBrowseEnabled: false,
						browseStatus: {
							pending: 0,
							queued: 0,
							active: 0,
							running: false,
						},
					},
				});
				return;
			}

			if (url.pathname === '/api/tree/root-0/images') {
				treeRequests++;
				await route.fulfill({
					json: [imageEntry(currentTags)],
				});
				return;
			}

			if (url.pathname === '/api/analyze/scan' && request.method() === 'POST') {
				analyzePayload = request.postDataJSON();
				currentTags = [{ label: freshTag, score: 0.95 }];
				await route.fulfill({
					json: {
						running: true,
						rootId: 'root-0',
						path: 'images',
						total: 1,
						completed: 0,
						errors: 0,
					},
				});
				return;
			}

			if (url.pathname === '/api/analyze/status') {
				await route.fulfill({
					json: {
						running: false,
						rootId: 'root-0',
						path: 'images',
						total: 1,
						completed: 1,
						errors: 0,
					},
				});
				return;
			}

			await route.fulfill({
				status: 404,
				body: `Unhandled ${request.method()} ${url.pathname}`,
			});
		});

		page.on('dialog', async (dialog) => {
			expect(dialog.message()).toContain('replacing all existing results');
			await dialog.accept();
		});

		await page.goto('/?root=root-0&path=images');

		const card = page.getByTestId('thumbnail-card');
		await expect(card).toHaveCount(1);
		await expect(card.getByText(legacyTag)).toBeVisible();

		await page.getByTitle(/Re-analyze this folder/).click();

		await expect
			.poll(() => analyzePayload)
			.toEqual({
				rootId: 'root-0',
				path: 'images',
				force: true,
			});
		await expect(card.getByText(freshTag)).toBeVisible({ timeout: 5000 });
		await expect(card.getByText(legacyTag)).not.toBeVisible();
		expect(treeRequests).toBeGreaterThanOrEqual(2);
	});
});

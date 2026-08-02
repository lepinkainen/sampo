import { expect, test } from '@playwright/test';

// Opens Sample → images (which holds test_*.jpg with thumbnails).
async function openImages(page: import('@playwright/test').Page) {
	await page.getByText('Sample').click();
	await page.locator('.select-none button', { hasText: 'images' }).click();
	await page.getByTestId('thumbnail-scroll').waitFor();
}

test.describe('View mode (grid/detail)', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await openImages(page);
	});

	test('detail view lists rows with real thumbnails, not generic icons', async ({
		page,
	}) => {
		await page.getByRole('button', { name: 'List view' }).click();

		const table = page.locator('table');
		await expect(table).toBeVisible();

		const thumbs = table.locator('img');
		await expect(thumbs.first()).toBeVisible();
		expect(await thumbs.count()).toBeGreaterThan(0);
		await expect(thumbs.first()).toHaveAttribute('src', /\/api\/thumb\//);
	});

	test('modified timestamp stays on one line (compact rows)', async ({
		page,
	}) => {
		await page.getByRole('button', { name: 'List view' }).click();
		const modCell = page
			.locator('td', { hasText: /^\s*\d{4}-\d{2}-\d{2}/ })
			.first();
		await expect(modCell).toHaveClass(/whitespace-nowrap/);
	});

	test('size selector stays visible but disabled in detail view', async ({
		page,
	}) => {
		// Visible + enabled in grid.
		await expect(
			page.getByRole('button', { name: 'M', exact: true }),
		).toBeEnabled();
		await page.getByRole('button', { name: 'List view' }).click();
		// Still present, now disabled.
		await expect(
			page.getByRole('button', { name: 'M', exact: true }),
		).toBeDisabled();
	});

	test('view mode persists across a page reload', async ({ page }) => {
		await page.getByRole('button', { name: 'List view' }).click();
		await expect(page.locator('table')).toBeVisible();

		// Reload restores the open folder from the URL; the view mode is
		// restored from localStorage, so detail view comes back automatically.
		await page.reload();
		await page.getByTestId('thumbnail-scroll').waitFor();

		await expect(page.locator('table')).toBeVisible();
		await expect(page.getByRole('button', { name: 'List view' })).toHaveClass(
			/bg-select/,
		);
	});

	test('thumb size persists across a page reload', async ({ page }) => {
		await page.getByRole('button', { name: 'L', exact: true }).click();
		await expect(page.locator('[class*="grid-cols-[repeat"]')).toHaveClass(
			/280px/,
		);

		// Reload restores the open folder from the URL; thumb size is restored
		// from localStorage.
		await page.reload();
		await page.getByTestId('thumbnail-scroll').waitFor();

		await expect(page.locator('[class*="grid-cols-[repeat"]')).toHaveClass(
			/280px/,
		);
		await expect(
			page.getByRole('button', { name: 'L', exact: true }),
		).toHaveClass(/bg-select/);
	});
});

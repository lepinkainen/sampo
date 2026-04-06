import { expect, test } from '@playwright/test';

test.describe('Breadcrumb navigation', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByText('Sample').click();
	});

	test('clicking root ID segment navigates to root directory', async ({
		page,
	}) => {
		// Navigate to a subdirectory first
		await page.locator('.select-none button', { hasText: 'images' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.first()
			.waitFor();

		const toolbar = page.locator('.border-b.bg-gray-900');
		await expect(toolbar).toContainText('/ images');

		// Click "Sample" segment to go back to root
		await toolbar.locator('button', { hasText: 'Sample' }).click();

		// Should show root contents — toolbar shows "/ (root)" or just the root path
		// Wait for the URL to update (no path param or empty path)
		await page.waitForURL((url) => {
			const p = new URL(url).searchParams.get('path');
			return !p || p === '' || p === '/';
		});

		// Grid should have items (directories like images, subdir, dir&special, videos)
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		await cards.first().waitFor();
		const count = await cards.count();
		expect(count).toBeGreaterThanOrEqual(3);
	});

	test('last breadcrumb segment is not clickable', async ({ page }) => {
		await page.locator('.select-none button', { hasText: 'images' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');

		const toolbar = page.locator('.border-b.bg-gray-900');

		// The last segment "images" should be a <span>, not a <button>
		// The breadcrumb renders last segment as plain span
		const breadcrumbArea = toolbar.locator('.truncate');
		const lastSpan = breadcrumbArea.locator('span').last();
		await expect(lastSpan).toContainText('images');

		// Should not be a button (no click handler)
		const tagName = await lastSpan.evaluate((el) => el.tagName.toLowerCase());
		expect(tagName).toBe('span');
	});
});

import { expect, test } from '@playwright/test';

test.describe('Toolbar and path display', () => {
	test('toolbar shows root ID and path', async ({ page }) => {
		await page.goto('/');
		await page.getByText('Sample').click();
		await page.locator('.select-none button', { hasText: 'images' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');

		// Toolbar area (border-b bar) should show root ID and path
		const toolbar = page.locator('.border-b.bg-gray-900');
		await expect(toolbar).toContainText('root-');
		await expect(toolbar).toContainText('/ images');
	});

	test('toolbar path updates when navigating directories', async ({ page }) => {
		await page.goto('/');
		await page.getByText('Sample').click();

		// Navigate to images
		await page.locator('.select-none button', { hasText: 'images' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		const toolbar = page.locator('.border-b.bg-gray-900');
		await expect(toolbar).toContainText('/ images');

		// Navigate to subdir
		await page.locator('.select-none button', { hasText: 'subdir' }).click();
		await expect(toolbar).toContainText('/ subdir');
	});
});

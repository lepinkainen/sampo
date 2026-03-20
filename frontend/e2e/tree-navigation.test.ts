import { expect, test } from '@playwright/test';

test.describe('Tree view and navigation', () => {
	test('page loads and shows roots in tree view', async ({ page }) => {
		await page.goto('/');

		// "File Manager" header should be visible
		await expect(page.getByText('File Manager')).toBeVisible();

		// "Sample" root should be visible in the tree
		await expect(page.getByText('Sample')).toBeVisible();

		// Placeholder text should show when no directory selected
		await expect(page.getByText('Select a directory to browse')).toBeVisible();
	});

	test('clicking a root expands it and shows subdirectories', async ({
		page,
	}) => {
		await page.goto('/');

		// Click "Sample" root to expand
		await page.getByText('Sample').click();

		// Should show child directories
		await expect(
			page.locator('.select-none button', { hasText: 'images' }),
		).toBeVisible();
		await expect(
			page.locator('.select-none button', { hasText: 'subdir' }),
		).toBeVisible();
	});

	test('clicking a directory updates the thumbnail grid', async ({ page }) => {
		await page.goto('/');
		await page.getByText('Sample').click();

		// Click "images" directory
		await page.locator('.select-none button', { hasText: 'images' }).click();

		// Placeholder should be gone
		await expect(
			page.getByText('Select a directory to browse'),
		).not.toBeVisible();

		// Thumbnail grid should appear
		await page.waitForSelector('[class*="grid-cols-[repeat"]');

		// URL should reflect the selection
		await page.waitForURL(/path=.*images/);
		const url = new URL(page.url());
		expect(url.searchParams.get('root')).toBeTruthy();
	});

	test('clicking a different directory switches grid contents', async ({
		page,
	}) => {
		await page.goto('/');
		await page.getByText('Sample').click();

		// Navigate to images
		await page.locator('.select-none button', { hasText: 'images' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');

		// Count thumbnails in images
		const imagesCount = await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.count();

		// Navigate to dir&special
		await page
			.locator('.select-none button', { hasText: 'dir&special' })
			.click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');

		// Count thumbnails in dir&special
		const specialCount = await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.count();

		// Both should have items but potentially different counts
		expect(imagesCount).toBeGreaterThan(0);
		expect(specialCount).toBeGreaterThan(0);

		// URL should reflect new directory
		await page.waitForURL(/path=.*dir/);
		const url = new URL(page.url());
		expect(url.searchParams.get('path')).toContain('dir');
	});
});

import { expect, test } from '@playwright/test';

test.describe('Special characters in paths', () => {
	test('ampersand in directory name shows thumbnails', async ({ page }) => {
		await page.goto('/');
		// Expand "Sample" root in tree view
		await page.getByText('Sample').click();
		// Click the tree node for dir&special (the button inside the tree, not the grid)
		await page
			.locator('.select-none button', { hasText: 'dir&special' })
			.click();
		// Wait for thumbnail grid
		await page.waitForSelector('[class*="grid"]');
		// Thumbnails should load (img elements with /api/thumb/ URLs)
		const thumbs = page.locator('img[src*="/api/thumb/"]');
		await expect(thumbs.first()).toBeVisible();
		const thumbCount = await thumbs.count();
		expect(thumbCount).toBeGreaterThan(0);
	});

	test('apostrophe in filename shows thumbnail', async ({ page }) => {
		await page.goto('/');
		// Expand "Sample" root in tree view
		await page.getByText('Sample').click();
		// Click the tree node for images directory
		await page.locator('.select-none button', { hasText: 'images' }).click();
		// Wait for thumbnail grid
		await page.waitForSelector('[class*="grid"]');
		// Find the card for the apostrophe file
		const card = page.locator('p', { hasText: "test'apostrophe.jpg" });
		await expect(card).toBeVisible();
		// The thumbnail image for this file should be loaded (not show error fallback)
		const parentCard = card.locator(
			'xpath=ancestor::div[contains(@class,"rounded-lg")]',
		);
		const img = parentCard.locator('img[src*="/api/thumb/"]');
		await expect(img).toBeVisible();
	});

	test('ampersand directory - preview opens and shows image', async ({
		page,
	}) => {
		await page.goto('/');
		await page.getByText('Sample').click();
		await page
			.locator('.select-none button', { hasText: 'dir&special' })
			.click();
		await page.waitForSelector('[class*="grid"]');
		// Double-click first thumbnail card to open preview
		const cards = page.locator('div.group[role="button"]');
		await cards.first().dblclick();
		// Preview should open with file URL
		const img = page.locator('img[src*="/api/file/"]');
		await expect(img).toBeVisible();
	});
});

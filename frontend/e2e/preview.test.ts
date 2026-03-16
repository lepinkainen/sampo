import { test, expect } from '@playwright/test';

test.describe('Media Preview', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		// Expand "Sample" root
		await page.getByText('Sample').click();
		// Navigate to images directory (use tree node selector to avoid grid matches)
		await page.locator('.select-none button', { hasText: 'images' }).click();
		// Wait for thumbnails to load
		await page.waitForSelector('[class*="grid"]');
	});

	test('clicking an image thumbnail opens preview', async ({ page }) => {
		// Click the first thumbnail card that has an image
		const card = page.locator('[role="button"]').first();
		await card.click();

		// Preview should show with close button and counter
		await expect(page.getByLabel('Close preview')).toBeVisible();
		await expect(page.getByText(/\d+ \/ \d+/)).toBeVisible();

		// An img element with the file API URL should be visible
		const img = page.locator('img[src*="/api/file/"]');
		await expect(img).toBeVisible();
	});

	test('arrow keys navigate between images', async ({ page }) => {
		// Open preview on first image
		const card = page.locator('[role="button"]').first();
		await card.click();

		// Should show "1 / N"
		await expect(page.getByText(/^1 \//)).toBeVisible();

		// Press right arrow
		await page.keyboard.press('ArrowRight');
		await expect(page.getByText(/^2 \//)).toBeVisible();

		// Press left arrow
		await page.keyboard.press('ArrowLeft');
		await expect(page.getByText(/^1 \//)).toBeVisible();
	});

	test('escape closes preview', async ({ page }) => {
		// Open preview
		const card = page.locator('[role="button"]').first();
		await card.click();
		await expect(page.getByLabel('Close preview')).toBeVisible();

		// Press escape
		await page.keyboard.press('Escape');

		// Preview should be gone, grid should be back
		await expect(page.getByLabel('Close preview')).not.toBeVisible();
		await expect(page.locator('[class*="grid"]')).toBeVisible();
	});

	test('close button closes preview', async ({ page }) => {
		// Open preview
		const card = page.locator('[role="button"]').first();
		await card.click();

		// Click close button
		await page.getByLabel('Close preview').click();

		// Grid should be back
		await expect(page.locator('[class*="grid"]')).toBeVisible();
	});
});

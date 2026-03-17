import { expect, test } from '@playwright/test';

test.describe('Media Preview', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		// Expand "Sample" root
		await page.getByText('Sample').click();
		// Navigate to images directory (use tree node selector to avoid grid matches)
		await page.locator('.select-none button', { hasText: 'images' }).click();
		// Wait for thumbnails to load
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
	});

	test('double-clicking an image thumbnail opens preview and updates URL', async ({
		page,
	}) => {
		// Double-click the first thumbnail card to open preview
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.dblclick();

		// Preview should show with close button and counter
		await expect(page.getByLabel('Close preview')).toBeVisible();
		await expect(page.getByText(/\d+ \/ \d+/)).toBeVisible();

		// URL should update
		const url = new URL(page.url());
		expect(url.searchParams.has('preview')).toBe(true);

		// An img element with the file API URL should be visible
		const img = page.locator('img[src*="/api/file/"]');
		await expect(img).toBeVisible();
	});

	test('arrow keys navigate between images and update URL', async ({
		page,
	}) => {
		// Open preview on first image
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.dblclick();

		const url1 = new URL(page.url());
		const preview1 = url1.searchParams.get('preview');

		// Should show "1 / N"
		await expect(page.getByText(/^1 \//)).toBeVisible();

		// Press right arrow
		await page.keyboard.press('ArrowRight');
		await expect(page.getByText(/^2 \//)).toBeVisible();

		// URL should change
		const url2 = new URL(page.url());
		const preview2 = url2.searchParams.get('preview');
		expect(preview2).not.toBe(preview1);

		// Press left arrow
		await page.keyboard.press('ArrowLeft');
		await expect(page.getByText(/^1 \//)).toBeVisible();

		// URL should return to first
		const url3 = new URL(page.url());
		expect(url3.searchParams.get('preview')).toBe(preview1);
	});

	test('escape closes preview and clears URL param', async ({ page }) => {
		// Open preview
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.dblclick();
		await expect(page.getByLabel('Close preview')).toBeVisible();
		expect(page.url()).toContain('preview=');

		// Press escape
		await page.keyboard.press('Escape');

		// Preview should be gone, grid should be back
		await expect(page.getByLabel('Close preview')).not.toBeVisible();
		await expect(page.locator('[class*="grid-cols-[repeat"]')).toBeVisible();
		expect(page.url()).not.toContain('preview=');
	});

	test('close button closes preview and clears URL param', async ({ page }) => {
		// Open preview
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.dblclick();
		expect(page.url()).toContain('preview=');

		// Click close button
		await page.getByLabel('Close preview').click();

		// Grid should be back
		await expect(page.locator('[class*="grid-cols-[repeat"]')).toBeVisible();
		expect(page.url()).not.toContain('preview=');
	});

	test('wrap-around navigation at boundaries', async ({ page }) => {
		// Open preview on first image
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.dblclick();
		await expect(page.getByText(/^1 \//)).toBeVisible();

		// Get total count from the counter text (e.g. "1 / 5")
		const counterText = await page.getByText(/^\d+ \/ \d+/).textContent();
		const total = parseInt(counterText!.split('/')[1].trim(), 10);

		// Press left arrow on first image — should wrap to last
		await page.keyboard.press('ArrowLeft');
		await expect(page.getByText(new RegExp(`^${total} /`))).toBeVisible();
		await expect(page.getByText('Wrapped to last')).toBeVisible();

		// Press right arrow on last image — should wrap to first
		await page.keyboard.press('ArrowRight');
		await expect(page.getByText(/^1 \//)).toBeVisible();
		await expect(page.getByText('Wrapped to first')).toBeVisible();
	});

	test('reopen preview after closing', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();

		// Open and close preview
		await card.dblclick();
		await expect(page.getByLabel('Close preview')).toBeVisible();
		await page.keyboard.press('Escape');
		await expect(page.getByLabel('Close preview')).not.toBeVisible();

		// Reopen same card
		await card.dblclick();
		await expect(page.getByLabel('Close preview')).toBeVisible();
		await expect(page.getByText(/^1 \//)).toBeVisible();
	});

	test('directory navigation clears preview', async ({ page }) => {
		// Open preview
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.dblclick();
		await expect(page.getByLabel('Close preview')).toBeVisible();

		// Click a different directory in the tree
		await page.locator('.select-none button', { hasText: 'videos' }).click();

		// Preview should be gone
		await expect(page.getByLabel('Close preview')).not.toBeVisible();
		expect(page.url()).not.toContain('preview=');
	});

	test('direct URL navigation opens preview', async ({ page }) => {
		// Open preview to get a valid URL
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.dblclick();
		const validUrl = page.url();

		// Navigate away
		await page.goto('/');
		await expect(page.getByLabel('Close preview')).not.toBeVisible();

		// Navigate back to the preview URL
		await page.goto(validUrl);

		// Preview should be open
		await expect(page.getByLabel('Close preview')).toBeVisible();
		await expect(page.getByText(/\d+ \/ \d+/)).toBeVisible();
	});
});

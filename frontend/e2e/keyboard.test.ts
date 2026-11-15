import { expect, test } from '@playwright/test';

test.describe('Keyboard navigation', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByText('Sample').click();
		await page.locator('.select-none button', { hasText: 'grid2x2' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
	});

	test('Space key on focused card selects it', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.focus();

		// Press Space to trigger click handler
		await page.keyboard.press('Space');

		// Card should be selected — details panel shows info
		await expect(
			page.getByText('Select a file to view details'),
		).not.toBeVisible();
		await expect(page.locator('.grid-cols-2').getByText('Size')).toBeVisible();
	});

	test('Enter key on focused card opens it', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.focus();

		await page.keyboard.press('Enter');

		await expect(page.getByLabel('Close preview')).toBeVisible();
		expect(new URL(page.url()).searchParams.has('preview')).toBe(true);
	});
});

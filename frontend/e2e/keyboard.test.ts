import { expect, test } from '@playwright/test';

test.describe('Keyboard navigation', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByText('Sample').click();
		await page.locator('.select-none button', { hasText: 'images' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
	});

	test('Enter key on focused card selects it', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();

		// Press Enter to trigger click handler
		await page.keyboard.press('Enter');

		// Card should be selected — details panel shows info
		await expect(
			page.getByText('Select a file to view details'),
		).not.toBeVisible();
		await expect(page.locator('.grid-cols-2').getByText('Size')).toBeVisible();
	});
});

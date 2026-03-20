import { expect, test } from '@playwright/test';

test.describe('Empty directory', () => {
	test('shows empty directory message for empty folder', async ({ page }) => {
		await page.goto('/');
		await page.getByText('Sample').click();

		// Click the "videos" directory which is empty in testdata
		await page.locator('.select-none button', { hasText: 'videos' }).click();

		// Should show empty directory message
		await expect(page.getByText('Empty directory')).toBeVisible();

		// Thumbnail grid should NOT be present
		await expect(
			page.locator('[class*="grid-cols-[repeat"]'),
		).not.toBeVisible();
	});
});

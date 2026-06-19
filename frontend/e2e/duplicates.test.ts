import { expect, test } from '@playwright/test';

test.describe('Duplicates modal', () => {
	test('opens and closes the duplicates modal', async ({ page }) => {
		await page.goto('/');
		await page.getByText('Sample').click();
		await page.locator('.select-none button', { hasText: 'images' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');

		// Open via the toolbar "Find duplicates" button.
		await page.getByTitle('Find duplicates').click();

		const modal = page.getByTestId('duplicates-modal');
		await expect(modal).toBeVisible();
		await expect(modal).toContainText('Duplicate Files');

		// Close with the X button inside the modal header.
		await modal.locator('button').first().click();
		await expect(modal).toHaveCount(0);
	});
});

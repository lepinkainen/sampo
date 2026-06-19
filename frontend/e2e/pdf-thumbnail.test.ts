import { expect, test } from '@playwright/test';

test.describe('PDF thumbnails', () => {
	test('renders a thumbnail image for a PDF file', async ({ page }) => {
		await page.goto('/');
		await page.getByText('Sample').click();
		await page.locator('.select-none button', { hasText: 'docs' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');

		// The PDF card should render an <img> thumbnail (not the file-type icon),
		// which means hasThumb was true and the backend produced a thumbnail.
		const thumb = page.locator('img[alt="sample.pdf"]');
		await expect(thumb).toBeVisible();
		await expect(thumb).toHaveJSProperty('naturalWidth', 300);
	});
});

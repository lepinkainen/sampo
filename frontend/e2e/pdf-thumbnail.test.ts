import { expect, test } from '@playwright/test';

test.describe('PDF thumbnails', () => {
	test('renders a thumbnail image for a PDF file', async ({ page }) => {
		await page.goto('/');
		await page.getByText('Sample').click();
		await page.locator('.select-none button', { hasText: 'docs' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');

		// The PDF card should render an <img> thumbnail (not the file-type icon),
		// which means hasThumb was true and the backend produced a thumbnail.
		// The img is decorative (alt=""), so find it through its card.
		const thumb = page
			.getByTestId('thumbnail-card')
			.filter({ hasText: 'sample.pdf' })
			.locator('img');
		await expect(thumb).toBeVisible();
		await expect(thumb).toHaveJSProperty('naturalWidth', 300);
	});
});

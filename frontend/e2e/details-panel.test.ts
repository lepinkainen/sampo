import { expect, test } from '@playwright/test';

test.describe('File selection and details panel', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByText('Sample').click();
		await page.locator('.select-none button', { hasText: 'images' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
	});

	test('details panel shows placeholder when no file selected', async ({
		page,
	}) => {
		await expect(page.getByText('Select a file to view details')).toBeVisible();
	});

	test('clicking a file shows its details', async ({ page }) => {
		// Click the first thumbnail card
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();

		// Placeholder should be gone
		await expect(
			page.getByText('Select a file to view details'),
		).not.toBeVisible();

		// Details panel should show file info
		// Size field should be visible (e.g. "1.2 KB")
		await expect(page.locator('.grid-cols-2').getByText('Size')).toBeVisible();
		await expect(
			page.locator('.grid-cols-2').getByText('Modified'),
		).toBeVisible();
		await expect(page.locator('.grid-cols-2').getByText('Path')).toBeVisible();

		// Open button should be visible
		await expect(page.getByRole('button', { name: 'Open' })).toBeVisible();
	});

	test('clicking a file highlights it with blue border', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();

		// Card should have selected styling
		await expect(card).toHaveClass(/border-blue-500/);
	});

	test('clicking empty area deselects file', async ({ page }) => {
		// Select a file
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();
		await expect(
			page.getByText('Select a file to view details'),
		).not.toBeVisible();

		// Click the empty scroll container area (outside the grid)
		await page
			.locator('.flex-1.overflow-y-auto')
			.click({ position: { x: 10, y: 10 } });

		// Details should go back to placeholder
		await expect(page.getByText('Select a file to view details')).toBeVisible();
	});
});

import { expect, test } from '@playwright/test';

test.describe('Grid size selector', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByText('Sample').click();
		await page.locator('.select-none button', { hasText: 'images' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
	});

	test('default grid size is medium', async ({ page }) => {
		// M button should have active styling
		const mediumBtn = page.getByRole('button', { name: 'M', exact: true });
		await expect(mediumBtn).toHaveClass(/bg-gray-700/);

		// Grid should use 180px columns
		const grid = page.locator('[class*="grid-cols-[repeat"]');
		await expect(grid).toHaveClass(/180px/);
	});

	test('clicking S changes grid to small columns', async ({ page }) => {
		await page.getByRole('button', { name: 'S', exact: true }).click();

		// Grid should use 120px columns
		const grid = page.locator('[class*="grid-cols-[repeat"]');
		await expect(grid).toHaveClass(/120px/);

		// S button should now be active
		await expect(
			page.getByRole('button', { name: 'S', exact: true }),
		).toHaveClass(/bg-gray-700/);
	});

	test('clicking L changes grid to large columns', async ({ page }) => {
		await page.getByRole('button', { name: 'L', exact: true }).click();

		// Grid should use 280px columns
		const grid = page.locator('[class*="grid-cols-[repeat"]');
		await expect(grid).toHaveClass(/280px/);

		// L button should now be active
		await expect(
			page.getByRole('button', { name: 'L', exact: true }),
		).toHaveClass(/bg-gray-700/);
	});
});

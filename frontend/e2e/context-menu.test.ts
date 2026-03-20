import { expect, test } from '@playwright/test';

test.describe('Context menu', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByText('Sample').click();
		await page.locator('.select-none button', { hasText: 'images' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
	});

	test('right-click shows context menu with all items', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click({ button: 'right' });

		const menu = page.locator('.fixed.z-50.min-w-\\[160px\\]');
		await expect(menu).toBeVisible();

		// All menu items should be present
		await expect(menu.getByText('Open')).toBeVisible();
		await expect(menu.getByText('Rename')).toBeVisible();
		await expect(menu.getByText('Cut')).toBeVisible();
		await expect(menu.getByText('Copy')).toBeVisible();
		await expect(menu.getByText('Paste')).toBeVisible();
		await expect(menu.getByText('Delete')).toBeVisible();
	});

	test('context menu disables Paste when clipboard is empty', async ({
		page,
	}) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click({ button: 'right' });

		const menu = page.locator('.fixed.z-50.min-w-\\[160px\\]');
		const pasteBtn = menu.locator('button', { hasText: 'Paste' });
		await expect(pasteBtn).toBeDisabled();
	});

	test('context menu enables Open and Rename for single selection', async ({
		page,
	}) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click({ button: 'right' });

		const menu = page.locator('.fixed.z-50.min-w-\\[160px\\]');
		const openBtn = menu.locator('button', { hasText: 'Open' });
		const renameBtn = menu.locator('button', { hasText: 'Rename' });
		await expect(openBtn).not.toBeDisabled();
		await expect(renameBtn).not.toBeDisabled();
	});

	test('context menu disables Open and Rename for multi-selection', async ({
		page,
	}) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');

		// Select two items
		await cards.nth(0).click();
		await cards.nth(1).click({ modifiers: ['ControlOrMeta'] });

		// Right-click on one of the selected items
		await cards.nth(1).click({ button: 'right' });

		const menu = page.locator('.fixed.z-50.min-w-\\[160px\\]');
		const openBtn = menu.locator('button', { hasText: 'Open' });
		const renameBtn = menu.locator('button', { hasText: 'Rename' });
		await expect(openBtn).toBeDisabled();
		await expect(renameBtn).toBeDisabled();

		// Cut, Copy, Delete should still be enabled
		await expect(menu.locator('button', { hasText: 'Cut' })).not.toBeDisabled();
		await expect(
			menu.locator('button', { hasText: 'Copy' }),
		).not.toBeDisabled();
		await expect(
			menu.locator('button', { hasText: 'Delete' }),
		).not.toBeDisabled();
	});

	test('Escape closes context menu', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click({ button: 'right' });

		const menu = page.locator('.fixed.z-50.min-w-\\[160px\\]');
		await expect(menu).toBeVisible();

		await page.keyboard.press('Escape');
		await expect(menu).not.toBeVisible();
	});

	test('clicking outside closes context menu', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click({ button: 'right' });

		const menu = page.locator('.fixed.z-50.min-w-\\[160px\\]');
		await expect(menu).toBeVisible();

		// Click the overlay (fixed inset-0 behind the menu)
		await page
			.locator('.fixed.inset-0.z-40')
			.click({ position: { x: 10, y: 10 } });
		await expect(menu).not.toBeVisible();
	});

	test('right-click on unselected item selects it', async ({ page }) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');

		// Select first card normally
		await cards.nth(0).click();
		await expect(cards.nth(0)).toHaveClass(/border-blue-500/);

		// Right-click second card (unselected)
		await cards.nth(1).click({ button: 'right' });

		// Second card should now be selected, first deselected
		await expect(cards.nth(1)).toHaveClass(/border-blue-500/);
		await expect(cards.nth(0)).not.toHaveClass(/border-blue-500/);
	});
});

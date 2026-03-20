import { expect, test } from '@playwright/test';

test.describe('Multi-selection', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByText('Sample').click();
		await page.locator('.select-none button', { hasText: 'images' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		// Wait for cards to actually render
		await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.first()
			.waitFor();
	});

	test('Ctrl+click toggles individual items', async ({ page }) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		const first = cards.nth(0);
		const second = cards.nth(1);

		// Ctrl+click first card
		await first.click({ modifiers: ['ControlOrMeta'] });
		await expect(first).toHaveClass(/border-blue-500/);

		// Ctrl+click second card — both should be selected
		await second.click({ modifiers: ['ControlOrMeta'] });
		await expect(first).toHaveClass(/border-blue-500/);
		await expect(second).toHaveClass(/border-blue-500/);

		// Toolbar shows selection count
		await expect(page.getByText('2 selected')).toBeVisible();

		// Ctrl+click first again — deselects it
		await first.click({ modifiers: ['ControlOrMeta'] });
		await expect(first).not.toHaveClass(/border-blue-500/);
		await expect(second).toHaveClass(/border-blue-500/);
		await expect(page.getByText('1 selected')).toBeVisible();
	});

	test('Shift+click selects a range', async ({ page }) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');

		// Click first card normally
		await cards.nth(0).click();
		await expect(cards.nth(0)).toHaveClass(/border-blue-500/);

		// Shift+click third card — should select range [0, 1, 2]
		await cards.nth(2).click({ modifiers: ['Shift'] });
		await expect(cards.nth(0)).toHaveClass(/border-blue-500/);
		await expect(cards.nth(1)).toHaveClass(/border-blue-500/);
		await expect(cards.nth(2)).toHaveClass(/border-blue-500/);

		await expect(page.getByText('3 selected')).toBeVisible();
	});

	test('Ctrl+A selects all items', async ({ page }) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		const totalCards = await cards.count();
		expect(totalCards).toBeGreaterThan(0);

		await page.keyboard.press('ControlOrMeta+a');

		// All cards should be selected
		await expect(page.getByText(`${totalCards} selected`)).toBeVisible();
		for (let i = 0; i < totalCards; i++) {
			await expect(cards.nth(i)).toHaveClass(/border-blue-500/);
		}
	});

	test('details panel shows multi-item summary', async ({ page }) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');

		// Select two items
		await cards.nth(0).click();
		await cards.nth(1).click({ modifiers: ['ControlOrMeta'] });

		// Details panel should show multi-item summary
		await expect(page.getByText('2 items selected')).toBeVisible();
		await expect(
			page.locator('.grid-cols-2').getByText('Total size'),
		).toBeVisible();
		await expect(page.locator('.grid-cols-2').getByText('Files')).toBeVisible();
		await expect(
			page.locator('.grid-cols-2').getByText('Folders'),
		).toBeVisible();
	});

	test('clicking empty area clears multi-selection', async ({ page }) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');

		// Select two items explicitly instead of Ctrl+A
		await cards.nth(0).click();
		await cards.nth(1).click({ modifiers: ['ControlOrMeta'] });
		await expect(page.getByText('2 selected')).toBeVisible();

		// Click empty area
		await page
			.locator('.flex-1.overflow-y-auto')
			.click({ position: { x: 10, y: 10 } });

		// Selection should be cleared
		await expect(page.getByText('selected')).not.toBeVisible();
		await expect(page.getByText('Select a file to view details')).toBeVisible();
	});
});

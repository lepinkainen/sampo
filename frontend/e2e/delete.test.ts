import { cpSync, mkdirSync, rmSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from '@playwright/test';
import { TESTDATA } from './testdata';

const TMPDIR = resolve(TESTDATA, '_tmp_delete');

test.describe('Delete files', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		// Create a temp directory with test files
		rmSync(TMPDIR, { recursive: true, force: true });
		mkdirSync(TMPDIR, { recursive: true });
		cpSync(
			resolve(TESTDATA, 'grid2x2/grid2x2_rgb_rrrr.png'),
			resolve(TMPDIR, 'grid2x2_rgb_rrrr.png'),
		);
		cpSync(
			resolve(TESTDATA, 'grid2x2/grid2x2_rgb_gggg.png'),
			resolve(TMPDIR, 'grid2x2_rgb_gggg.png'),
		);
		cpSync(
			resolve(TESTDATA, 'grid2x2/grid2x2_rgb_bbbb.png'),
			resolve(TMPDIR, 'grid2x2_rgb_bbbb.png'),
		);
		if (testInfo.title.includes('preserves thumbnail scroll')) {
			const colours = ['r', 'g', 'b'];
			for (let i = 0; i < 18; i++) {
				let n = i;
				const pattern = Array.from({ length: 4 }, () => {
					const colour = colours[n % colours.length];
					n = Math.floor(n / colours.length);
					return colour;
				}).join('');
				const filename = `grid2x2_rgb_${pattern}.png`;
				cpSync(
					resolve(TESTDATA, 'grid2x2', filename),
					resolve(TMPDIR, `extra_${filename}`),
				);
			}
		}

		await page.goto('/');
		await page.getByText('Sample').click();
		// Wait for tree to expand and show the temp directory
		const treeNode = page.locator('.select-none button', {
			hasText: '_tmp_delete',
		});
		await treeNode.waitFor({ timeout: 5000 });
		await treeNode.click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.first()
			.waitFor();
	});

	test.afterEach(() => {
		rmSync(TMPDIR, { recursive: true, force: true });
	});

	test('delete button opens confirm dialog', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();

		// Click the delete toolbar button
		await page.locator('button[title="Delete"]').click();

		// Confirm dialog should appear
		const dialog = page.locator('dialog[open]');
		await expect(dialog).toBeVisible();
		await expect(dialog.getByText('Delete 1 item(s)?')).toBeVisible();

		// Item name should be listed
		await expect(dialog.locator('.bg-canvas')).toBeVisible();
	});

	test('cancel keeps file intact', async ({ page }) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		const countBefore = await cards.count();

		await cards.first().click();
		await page.locator('button[title="Delete"]').click();

		// Click Cancel
		await page
			.locator('dialog[open]')
			.locator('button', { hasText: 'Cancel' })
			.click();

		// Dialog should close
		await expect(page.locator('dialog[open]')).not.toBeVisible();

		// File count should be unchanged
		expect(await cards.count()).toBe(countBefore);
	});

	test('confirm deletes file and shows toast', async ({ page }) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		const countBefore = await cards.count();

		await cards.first().click();
		await page.locator('button[title="Delete"]').click();

		// Click Delete to confirm
		const dialog = page.locator('dialog[open]');
		await dialog.locator('button', { hasText: 'Delete' }).click();

		// Dialog should close
		await expect(dialog).not.toBeVisible();

		// Toast should show
		await expect(page.getByText('Deleted 1 item(s)')).toBeVisible();

		// Grid should have one fewer item
		await expect(cards).toHaveCount(countBefore - 1);
	});

	test('confirm delete preserves thumbnail scroll position', async ({
		page,
	}) => {
		await page.setViewportSize({ width: 520, height: 320 });

		const scroller = page.getByTestId('thumbnail-scroll');
		const cards = page.getByTestId('thumbnail-card');
		const countBefore = await cards.count();
		await cards.nth(12).scrollIntoViewIfNeeded();
		expect(await scroller.evaluate((el) => el.scrollTop)).toBeGreaterThan(0);

		await cards.nth(12).click();
		await page.locator('button[title="Delete"]').click();
		const dialog = page.locator('dialog[open]');
		await expect(dialog).toBeVisible();
		const savedScrollTop = await scroller.evaluate((el) => el.scrollTop);
		expect(savedScrollTop).toBeGreaterThan(0);
		await dialog.locator('button', { hasText: 'Delete' }).click();

		await expect(cards).toHaveCount(countBefore - 1, { timeout: 15000 });
		await expect
			.poll(async () => scroller.evaluate((el) => el.scrollTop))
			.toBeGreaterThan(savedScrollTop - 200);
	});

	test('Delete key shortcut opens confirm dialog', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();

		await page.keyboard.press('Delete');

		const dialog = page.locator('dialog[open]');
		await expect(dialog).toBeVisible();
		await expect(dialog.getByText('Delete 1 item(s)?')).toBeVisible();
	});

	test('multi-select delete shows count in dialog', async ({ page }) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');

		// Select all items
		await page.keyboard.press('ControlOrMeta+a');
		const totalCards = await cards.count();

		await page.keyboard.press('Delete');

		const dialog = page.locator('dialog[open]');
		await expect(
			dialog.getByText(`Delete ${totalCards} item(s)?`),
		).toBeVisible();

		// All item names should be listed
		const itemList = dialog.locator('.bg-canvas p');
		await expect(itemList).toHaveCount(totalCards);
	});

	test('delete multiple files removes them all', async ({ page }) => {
		// Select all
		await page.keyboard.press('ControlOrMeta+a');

		await page.keyboard.press('Delete');

		// Confirm
		const dialog = page.locator('dialog[open]');
		await dialog.locator('button', { hasText: 'Delete' }).click();

		// Toast
		await expect(page.getByText(/Deleted \d+ item/)).toBeVisible();

		// Grid should show empty directory
		await expect(page.getByText('Empty directory')).toBeVisible();
	});

	test('Enter on the delete dialog cancels (focus starts on Cancel)', async ({
		page,
	}) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		const countBefore = await cards.count();

		await cards.first().click();
		await page.locator('button[title="Delete"]').click();

		const dialog = page.locator('dialog[open]');
		await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeFocused();

		// A stray Enter must take the safe action, not delete.
		await page.keyboard.press('Enter');
		await expect(dialog).not.toBeVisible();
		await expect(cards).toHaveCount(countBefore);
	});

	test('keyboard can confirm delete via the Delete button', async ({
		page,
	}) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		const countBefore = await cards.count();

		await cards.first().click();
		await page.locator('button[title="Delete"]').click();

		const dialog = page.locator('dialog[open]');
		await expect(dialog).toBeVisible();
		await page.keyboard.press('Tab');
		await expect(dialog.getByRole('button', { name: 'Delete' })).toBeFocused();
		await page.keyboard.press('Enter');

		await expect(page.getByText('Deleted 1 item(s)')).toBeVisible();
		await expect(cards).toHaveCount(countBefore - 1);
	});

	test('Delete key while typing in search does not open the dialog', async ({
		page,
	}) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		await cards.first().click();

		await page.locator('button[title="Search (Ctrl+F)"]').click();
		const search = page.getByPlaceholder('Search files and tags...');
		await expect(search).toBeFocused();
		await search.pressSequentially('a');
		await page.keyboard.press('Delete');
		await page.keyboard.press('ControlOrMeta+Backspace');

		await expect(page.locator('dialog[open]')).toHaveCount(0);
	});
});

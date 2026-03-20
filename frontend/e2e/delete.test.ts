import { cpSync, mkdirSync, rmSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test } from '@playwright/test';

const TESTDATA = resolve(
	dirname(fileURLToPath(import.meta.url)),
	'../../testdata',
);
const TMPDIR = resolve(TESTDATA, '_tmp_delete');

test.describe('Delete files', () => {
	test.beforeEach(async ({ page }) => {
		// Create a temp directory with test files
		rmSync(TMPDIR, { recursive: true, force: true });
		mkdirSync(TMPDIR, { recursive: true });
		cpSync(
			resolve(TESTDATA, 'images/test_red.jpg'),
			resolve(TMPDIR, 'test_red.jpg'),
		);
		cpSync(
			resolve(TESTDATA, 'images/test_blue.jpg'),
			resolve(TMPDIR, 'test_blue.jpg'),
		);
		cpSync(
			resolve(TESTDATA, 'images/test_green.jpg'),
			resolve(TMPDIR, 'test_green.jpg'),
		);

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
		const dialog = page.locator('.fixed.inset-0.z-50');
		await expect(dialog).toBeVisible();
		await expect(dialog.getByText('Delete 1 item(s)?')).toBeVisible();

		// Item name should be listed
		await expect(dialog.locator('.bg-gray-950')).toBeVisible();
	});

	test('cancel keeps file intact', async ({ page }) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		const countBefore = await cards.count();

		await cards.first().click();
		await page.locator('button[title="Delete"]').click();

		// Click Cancel
		await page
			.locator('.fixed.inset-0.z-50')
			.locator('button', { hasText: 'Cancel' })
			.click();

		// Dialog should close
		await expect(page.locator('.fixed.inset-0.z-50')).not.toBeVisible();

		// File count should be unchanged
		expect(await cards.count()).toBe(countBefore);
	});

	test('confirm deletes file and shows toast', async ({ page }) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		const countBefore = await cards.count();

		await cards.first().click();
		await page.locator('button[title="Delete"]').click();

		// Click Delete to confirm
		const dialog = page.locator('.fixed.inset-0.z-50');
		await dialog.locator('button', { hasText: 'Delete' }).click();

		// Dialog should close
		await expect(dialog).not.toBeVisible();

		// Toast should show
		await expect(page.getByText('Deleted 1 item(s)')).toBeVisible();

		// Grid should have one fewer item
		await expect(cards).toHaveCount(countBefore - 1);
	});

	test('Delete key shortcut opens confirm dialog', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();

		await page.keyboard.press('Delete');

		const dialog = page.locator('.fixed.inset-0.z-50');
		await expect(dialog).toBeVisible();
		await expect(dialog.getByText('Delete 1 item(s)?')).toBeVisible();
	});

	test('multi-select delete shows count in dialog', async ({ page }) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');

		// Select all items
		await page.keyboard.press('ControlOrMeta+a');
		const totalCards = await cards.count();

		await page.keyboard.press('Delete');

		const dialog = page.locator('.fixed.inset-0.z-50');
		await expect(
			dialog.getByText(`Delete ${totalCards} item(s)?`),
		).toBeVisible();

		// All item names should be listed
		const itemList = dialog.locator('.bg-gray-950 p');
		await expect(itemList).toHaveCount(totalCards);
	});

	test('delete multiple files removes them all', async ({ page }) => {
		// Select all
		await page.keyboard.press('ControlOrMeta+a');

		await page.keyboard.press('Delete');

		// Confirm
		const dialog = page.locator('.fixed.inset-0.z-50');
		await dialog.locator('button', { hasText: 'Delete' }).click();

		// Toast
		await expect(page.getByText(/Deleted \d+ item/)).toBeVisible();

		// Grid should show empty directory
		await expect(page.getByText('Empty directory')).toBeVisible();
	});

	test('Enter key confirms delete dialog', async ({ page }) => {
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		const countBefore = await cards.count();

		await cards.first().click();
		await page.locator('button[title="Delete"]').click();

		// Press Enter to confirm
		await page.keyboard.press('Enter');

		// File should be deleted
		await expect(page.getByText('Deleted 1 item(s)')).toBeVisible();
		await expect(cards).toHaveCount(countBefore - 1);
	});
});

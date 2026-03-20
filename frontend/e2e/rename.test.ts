import { cpSync, mkdirSync, rmSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test } from '@playwright/test';

const TESTDATA = resolve(
	dirname(fileURLToPath(import.meta.url)),
	'../../testdata',
);
const TMPDIR = resolve(TESTDATA, '_tmp_rename');

test.describe('Rename files', () => {
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

		await page.goto('/');
		await page.getByText('Sample').click();
		// Wait for tree to expand and show the temp directory
		const treeNode = page.locator('.select-none button', {
			hasText: '_tmp_rename',
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

	test('F2 opens rename dialog with pre-filled name', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();

		await page.keyboard.press('F2');

		// Rename dialog should appear
		const dialog = page.locator('.fixed.inset-0.z-50');
		await expect(dialog).toBeVisible();
		await expect(dialog.getByRole('heading', { name: 'Rename' })).toBeVisible();

		// Input should be pre-filled with current filename
		const input = dialog.locator('input[type="text"]');
		const value = await input.inputValue();
		expect(value).toMatch(/test_(red|blue)\.jpg/);
	});

	test('rename dialog selects name without extension', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();
		await page.keyboard.press('F2');

		const input = page.locator('.fixed.inset-0.z-50 input[type="text"]');
		const value = await input.inputValue();

		// Check selection range excludes extension
		const selStart = await input.evaluate(
			(el: HTMLInputElement) => el.selectionStart,
		);
		const selEnd = await input.evaluate(
			(el: HTMLInputElement) => el.selectionEnd,
		);
		const dotIndex = value.lastIndexOf('.');
		expect(selStart).toBe(0);
		expect(selEnd).toBe(dotIndex);
	});

	test('rename button disabled when name unchanged or empty', async ({
		page,
	}) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();
		await page.keyboard.press('F2');

		const dialog = page.locator('.fixed.inset-0.z-50');
		const renameBtn = dialog.locator('button', { hasText: 'Rename' });

		// Button should be disabled (name unchanged)
		await expect(renameBtn).toBeDisabled();

		// Clear input — still disabled
		const input = dialog.locator('input[type="text"]');
		await input.fill('');
		await expect(renameBtn).toBeDisabled();

		// Type a new name — enabled
		await input.fill('new_name.jpg');
		await expect(renameBtn).not.toBeDisabled();
	});

	test('Escape cancels rename dialog', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();
		await page.keyboard.press('F2');

		const dialog = page.locator('.fixed.inset-0.z-50');
		await expect(dialog).toBeVisible();

		await page.keyboard.press('Escape');
		await expect(dialog).not.toBeVisible();
	});

	test('successful rename updates grid and shows toast', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();
		await page.keyboard.press('F2');

		const dialog = page.locator('.fixed.inset-0.z-50');
		const input = dialog.locator('input[type="text"]');

		await input.fill('renamed_file.jpg');
		await dialog.locator('button', { hasText: 'Rename' }).click();

		// Dialog should close
		await expect(dialog).not.toBeVisible();

		// Toast should show success
		await expect(page.getByText('Renamed to "renamed_file.jpg"')).toBeVisible();

		// Grid should show new filename (use grid locator to avoid matching toast text)
		await expect(
			page.locator('[class*="grid-cols"] > [role="button"]', {
				hasText: 'renamed_file.jpg',
			}),
		).toBeVisible();
	});

	test('Enter confirms rename', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();
		await page.keyboard.press('F2');

		const input = page.locator('.fixed.inset-0.z-50 input[type="text"]');
		await input.fill('enter_rename.jpg');
		await page.keyboard.press('Enter');

		// Toast should confirm rename
		await expect(page.getByText('Renamed to "enter_rename.jpg"')).toBeVisible();
	});

	test('toolbar rename button works', async ({ page }) => {
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();

		// Click the rename toolbar button (Pencil icon, title="Rename (F2)")
		await page.locator('button[title="Rename (F2)"]').click();

		const dialog = page.locator('.fixed.inset-0.z-50');
		await expect(dialog).toBeVisible();
		await expect(dialog.getByRole('heading', { name: 'Rename' })).toBeVisible();
	});
});

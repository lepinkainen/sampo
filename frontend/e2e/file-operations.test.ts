import { cpSync, mkdirSync, rmSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test } from '@playwright/test';

const TESTDATA = resolve(
	dirname(fileURLToPath(import.meta.url)),
	'../../testdata',
);
const SRC_DIR = resolve(TESTDATA, '_tmp_ops_src');
const DST_DIR = resolve(TESTDATA, '_tmp_ops_dst');

test.describe('File operations: Cut/Copy/Paste', () => {
	test.beforeEach(async ({ page }) => {
		// Create source dir with test files and empty destination dir
		rmSync(SRC_DIR, { recursive: true, force: true });
		rmSync(DST_DIR, { recursive: true, force: true });
		mkdirSync(SRC_DIR, { recursive: true });
		mkdirSync(DST_DIR, { recursive: true });
		cpSync(
			resolve(TESTDATA, 'images/test_red.jpg'),
			resolve(SRC_DIR, 'test_red.jpg'),
		);
		cpSync(
			resolve(TESTDATA, 'images/test_blue.jpg'),
			resolve(SRC_DIR, 'test_blue.jpg'),
		);
		cpSync(
			resolve(TESTDATA, 'images/test_green.jpg'),
			resolve(SRC_DIR, 'test_green.jpg'),
		);

		await page.goto('/');
		await page.getByText('Sample').click();
		// Wait for tree to expand
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_src' })
			.waitFor({ timeout: 5000 });
	});

	test.afterEach(() => {
		rmSync(SRC_DIR, { recursive: true, force: true });
		rmSync(DST_DIR, { recursive: true, force: true });
	});

	test('Ctrl+C copies and Ctrl+V pastes files to another directory', async ({
		page,
	}) => {
		// Navigate to source directory
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_src' })
			.click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.first()
			.waitFor();

		// Select a file and copy
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		await cards.first().click();
		await page.keyboard.press('ControlOrMeta+c');
		await expect(page.getByText('Copied 1 item(s)')).toBeVisible();

		// Wait for copy toast to disappear before pasting (avoids duplicate text match)
		await expect(page.getByText('Copied 1 item(s)')).not.toBeVisible({
			timeout: 5000,
		});

		// Navigate to destination directory
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_dst' })
			.click();
		await expect(page.getByText('Empty directory')).toBeVisible();

		// Paste
		await page.keyboard.press('ControlOrMeta+v');

		// Destination should now have the file
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		await expect(cards).toHaveCount(1);

		// Navigate back to source — file should still be there
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_src' })
			.click();
		await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.first()
			.waitFor();
		await expect(cards).toHaveCount(3);
	});

	test('Ctrl+X cuts and Ctrl+V moves files (removes from source)', async ({
		page,
	}) => {
		// Navigate to source directory
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_src' })
			.click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.first()
			.waitFor();

		// Select a file and cut
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		await cards.first().click();
		await page.keyboard.press('ControlOrMeta+x');
		await expect(page.getByText('Cut 1 item(s)')).toBeVisible();

		// Navigate to destination
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_dst' })
			.click();
		await expect(page.getByText('Empty directory')).toBeVisible();

		// Paste
		await page.keyboard.press('ControlOrMeta+v');
		await expect(page.getByText('Moved 1 item(s)')).toBeVisible();

		// Destination should have the file
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		await expect(cards).toHaveCount(1);

		// Source should have one fewer file
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_src' })
			.click();
		await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.first()
			.waitFor();
		await expect(cards).toHaveCount(2);
	});

	test('cut items show reduced opacity', async ({ page }) => {
		// Navigate to source directory
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_src' })
			.click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.first()
			.waitFor();

		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		await cards.first().click();
		await page.keyboard.press('ControlOrMeta+x');

		// Cut card should have opacity-50 class
		await expect(cards.first()).toHaveClass(/opacity-50/);

		// Other cards should not have reduced opacity
		await expect(cards.nth(1)).not.toHaveClass(/opacity-50/);
	});

	test('toolbar copy button works', async ({ page }) => {
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_src' })
			.click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.first()
			.waitFor();

		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		await cards.first().click();

		// Click toolbar copy button
		await page.locator('button[title="Copy (Ctrl+C)"]').click();
		await expect(page.getByText('Copied 1 item(s)')).toBeVisible();
	});

	test('toolbar cut button works', async ({ page }) => {
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_src' })
			.click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.first()
			.waitFor();

		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		await cards.first().click();

		// Click toolbar cut button
		await page.locator('button[title="Cut (Ctrl+X)"]').click();
		await expect(page.getByText('Cut 1 item(s)')).toBeVisible();
		await expect(cards.first()).toHaveClass(/opacity-50/);
	});

	test('toolbar paste button works', async ({ page }) => {
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_src' })
			.click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.first()
			.waitFor();

		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		await cards.first().click();
		await page.keyboard.press('ControlOrMeta+c');
		await expect(page.getByText('Copied 1 item(s)')).toBeVisible();

		// Wait for copy toast to disappear
		await expect(page.getByText('Copied 1 item(s)')).not.toBeVisible({
			timeout: 5000,
		});

		// Navigate to destination
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_dst' })
			.click();
		await expect(page.getByText('Empty directory')).toBeVisible();

		// Click toolbar paste button
		await page.locator('button[title="Paste (Ctrl+V)"]').click();

		// File should appear in destination
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		await expect(cards).toHaveCount(1);
	});

	test('toolbar buttons disabled states', async ({ page }) => {
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_src' })
			.click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.first()
			.waitFor();

		// No selection: cut, copy, delete disabled; paste disabled (no clipboard)
		await expect(page.locator('button[title="Cut (Ctrl+X)"]')).toBeDisabled();
		await expect(page.locator('button[title="Copy (Ctrl+C)"]')).toBeDisabled();
		await expect(page.locator('button[title="Paste (Ctrl+V)"]')).toBeDisabled();
		await expect(page.locator('button[title="Delete"]')).toBeDisabled();
		await expect(page.locator('button[title="Rename (F2)"]')).toBeDisabled();

		// Select one item: cut, copy, delete, rename enabled; paste still disabled
		const card = page.locator('[class*="grid-cols"] > [role="button"]').first();
		await card.click();
		await expect(
			page.locator('button[title="Cut (Ctrl+X)"]'),
		).not.toBeDisabled();
		await expect(
			page.locator('button[title="Copy (Ctrl+C)"]'),
		).not.toBeDisabled();
		await expect(page.locator('button[title="Delete"]')).not.toBeDisabled();
		await expect(
			page.locator('button[title="Rename (F2)"]'),
		).not.toBeDisabled();
		await expect(page.locator('button[title="Paste (Ctrl+V)"]')).toBeDisabled();

		// Copy to clipboard: paste becomes enabled
		await page.keyboard.press('ControlOrMeta+c');
		await expect(
			page.locator('button[title="Paste (Ctrl+V)"]'),
		).not.toBeDisabled();
	});

	test('copy multiple files at once', async ({ page }) => {
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_src' })
			.click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		await page
			.locator('[class*="grid-cols"] > [role="button"]')
			.first()
			.waitFor();

		// Select all files
		const cards = page.locator('[class*="grid-cols"] > [role="button"]');
		await expect(cards).toHaveCount(3);
		await page.keyboard.press('ControlOrMeta+a');
		await expect(page.getByText('3 selected')).toBeVisible();

		await page.keyboard.press('ControlOrMeta+c');
		await expect(page.getByText('Copied 3 item(s)')).toBeVisible();

		// Wait for toast to disappear
		await expect(page.getByText('Copied 3 item(s)')).not.toBeVisible({
			timeout: 5000,
		});

		// Navigate to destination and paste
		await page
			.locator('.select-none button', { hasText: '_tmp_ops_dst' })
			.click();
		await expect(page.getByText('Empty directory')).toBeVisible();

		await page.keyboard.press('ControlOrMeta+v');

		// All files should be in destination
		await page.waitForSelector('[class*="grid-cols-[repeat"]');
		await expect(cards).toHaveCount(3);
	});
});

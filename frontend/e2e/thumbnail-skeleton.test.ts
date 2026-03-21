import { expect, test } from '@playwright/test';

test.describe('Thumbnail skeleton loading states', () => {
	test('shows shimmer skeletons in grid cards and details panel while thumbs load', async ({
		page,
	}) => {
		let releaseThumbs: () => void;
		const thumbsReleased = new Promise<void>((resolve) => {
			releaseThumbs = resolve;
		});

		await page.route('**/api/thumb/**', async (route) => {
			await thumbsReleased;
			await route.continue();
		});

		await page.goto('/');
		await page.getByText('Sample').click();
		await page.locator('.select-none button', { hasText: 'images' }).click();
		await page.waitForSelector('[class*="grid-cols-[repeat"]');

		const gridSkeletons = page.getByTestId('thumbnail-skeleton');
		await expect.poll(() => gridSkeletons.count()).toBeGreaterThan(0);

		const firstCard = page.getByTestId('thumbnail-card').first();
		await firstCard.click();

		const detailsSkeleton = page.getByTestId('details-thumbnail-skeleton');
		await expect(detailsSkeleton).toHaveCount(1);

		releaseThumbs!();

		await expect(gridSkeletons).toHaveCount(0, { timeout: 5000 });
		await expect(detailsSkeleton).toHaveCount(0, { timeout: 5000 });
	});
});

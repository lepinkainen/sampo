import { defineConfig } from '@playwright/test';

export default defineConfig({
	testDir: 'e2e',
	globalSetup: './e2e/global-setup.ts',
	// The isolated Docker backend (docker-compose.e2e.yml) starts with a cold
	// thumbnail cache, so cards lazy-load slower than against the warm dev
	// server the specs were tuned on. A couple of card-count/selection asserts
	// race that first paint intermittently (a different one each run). Retry
	// flaky specs and give expects more slack rather than sprinkle waits.
	retries: 2,
	expect: { timeout: 10_000 },
	use: {
		// Defaults to the isolated e2e Docker stack (docker-compose.e2e.yml on
		// :8091). `task test-e2e` sets PLAYWRIGHT_BASE_URL; override it to point
		// at a manually-run server if needed.
		baseURL: process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:8091',
	},
	projects: [
		{
			name: 'chromium',
			use: { browserName: 'chromium' },
		},
	],
});

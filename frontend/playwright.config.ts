import { defineConfig } from '@playwright/test';

export default defineConfig({
	testDir: 'e2e',
	globalSetup: './e2e/global-setup.ts',
	use: {
		baseURL: 'http://localhost:8080',
	},
	projects: [
		{
			name: 'chromium',
			use: { browserName: 'chromium' },
		},
	],
});

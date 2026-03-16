import { defineConfig } from '@playwright/test';

export default defineConfig({
	testDir: 'e2e',
	use: {
		baseURL: 'http://localhost:8080'
	},
	projects: [
		{
			name: 'chromium',
			use: { browserName: 'chromium' }
		}
	]
});

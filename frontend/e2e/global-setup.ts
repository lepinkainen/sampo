import { type FullConfig, request } from '@playwright/test';

export default async function globalSetup(config: FullConfig) {
	const baseURL = config.projects[0]?.use.baseURL ?? config.use.baseURL;
	if (!baseURL) {
		throw new Error('Playwright baseURL is required to disable auto-analysis');
	}

	// Disable auto-analysis so browsing doesn't kick off ML work mid-test. The
	// e2e backend is a throwaway Docker container (docker-compose.e2e.yml), so
	// there's no user preference to preserve or restore.
	const api = await request.newContext({ baseURL });
	try {
		const response = await api.post('/api/analysis/settings', {
			data: { autoBrowseEnabled: false },
		});
		if (!response.ok()) {
			throw new Error(
				`Failed to disable auto-analysis: ${response.status()} ${await response.text()}`,
			);
		}
	} finally {
		await api.dispose();
	}
}

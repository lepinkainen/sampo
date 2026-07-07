import { type FullConfig, request } from '@playwright/test';

export default async function globalSetup(config: FullConfig) {
	const baseURL = config.projects[0]?.use.baseURL ?? config.use.baseURL;
	if (!baseURL) {
		throw new Error('Playwright baseURL is required to disable auto-analysis');
	}

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

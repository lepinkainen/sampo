import { type FullConfig, request } from '@playwright/test';

export default async function globalSetup(config: FullConfig) {
	const baseURL = config.projects[0]?.use.baseURL ?? config.use.baseURL;
	if (!baseURL) {
		throw new Error('Playwright baseURL is required to disable auto-analysis');
	}

	const api = await request.newContext({ baseURL });
	try {
		// Capture the user's current preference so global-teardown can restore it;
		// otherwise the e2e run leaves the shared dev backend with auto-analysis off.
		const before = await api.get('/api/analysis/settings');
		if (before.ok()) {
			const settings = (await before.json()) as { autoBrowseEnabled?: boolean };
			process.env.SAMPO_PREV_AUTO_BROWSE = settings.autoBrowseEnabled
				? '1'
				: '0';
		}

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

import { type FullConfig, request } from '@playwright/test';

export default async function globalTeardown(_config: FullConfig) {
	const baseURL = _config.projects[0]?.use.baseURL ?? _config.use.baseURL;
	if (!baseURL) {
		return;
	}

	const restore = process.env.SAMPO_PREV_AUTO_BROWSE === '1';
	const api = await request.newContext({ baseURL });
	try {
		await api.post('/api/analysis/settings', {
			data: { autoBrowseEnabled: restore },
		});
	} finally {
		await api.dispose();
	}
}

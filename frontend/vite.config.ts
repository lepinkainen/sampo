import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		watch: {
			ignored: [
				'**/node_modules/**',
				'**/.git/**',
				'**/build/**',
				'**/.svelte-kit/**',
				'**/e2e/**',
				'**/*.test.ts',
				'**/*.spec.ts',
			],
		},
		proxy: {
			'/api': 'http://localhost:8080',
			'/whoami': 'http://localhost:8080',
			'/health': 'http://localhost:8080',
		},
	},
});

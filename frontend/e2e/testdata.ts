import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

/**
 * Host-side path to the e2e testdata root.
 *
 * Defaults to the per-run sandbox `.run/e2e-testdata/` that `task test-e2e`
 * creates (by copying `testdata/`) and bind-mounts into the isolated Docker
 * stack — specs read/write here so the checked-in `testdata/` is never
 * mutated. Override with SAMPO_E2E_TESTDATA when running specs against a
 * manually-run server whose root lives elsewhere, e.g.:
 *   SAMPO_E2E_TESTDATA=./testdata PLAYWRIGHT_BASE_URL=http://localhost:8080
 */
export const TESTDATA = process.env.SAMPO_E2E_TESTDATA
	? resolve(process.env.SAMPO_E2E_TESTDATA)
	: resolve(dirname(fileURLToPath(import.meta.url)), '../../.run/e2e-testdata');

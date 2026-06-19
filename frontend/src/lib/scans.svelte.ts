import type { ScanStatus } from './api';

export type ToastKind = 'success' | 'error';

export interface ScanOptions {
	/** Kick off the scan on the backend. */
	start: (rootId: string, path: string) => Promise<ScanStatus>;
	/** Poll current scan status. */
	poll: () => Promise<ScanStatus>;
	/** Toast shown when the scan starts. */
	startMsg: (s: ScanStatus) => string;
	/** Toast shown when the scan finishes. */
	doneMsg: (s: ScanStatus) => string;
	/** Fallback error message if start throws a non-Error. */
	errMsg: string;
	/** Emit a toast message. */
	onToast: (msg: string, kind: ToastKind) => void;
	/** Called once the scan completes (e.g. invalidate cache + reload). */
	onComplete: (rootId: string, path: string) => void;
	/** Poll interval in ms (default 1000). */
	pollMs?: number;
}

export interface ReloadAfterScanOptions {
	/** Invalidate the cache for the scanned path. */
	invalidate: (rootId: string, path: string) => void;
	/** Reload directory entries for the given path. */
	reload: (rootId: string, path: string) => void;
	/** Resolve the folder currently being viewed. */
	current: () => { rootId: string; path: string };
}

/**
 * makeReloadAfterScan builds a scan `onComplete` handler that avoids clobbering
 * the current folder when a scan finishes after the user navigated away. The
 * component instance is reused across path changes, so a late poll completion
 * must not replace the now-current folder's entries with the scanned folder's.
 *
 * The scanned path's cache is always invalidated (when invalidateCache), but the
 * directory is only reloaded if the scanned path is still the one on screen.
 */
export function makeReloadAfterScan(
	opts: ReloadAfterScanOptions,
	invalidateCache = true,
) {
	return (rootId: string, path: string) => {
		if (invalidateCache) opts.invalidate(rootId, path);
		const cur = opts.current();
		if (cur.rootId === rootId && cur.path === path) {
			opts.reload(rootId, path);
		}
	};
}

/**
 * createScan dedups the start → toast → poll-until-done → reload flow shared by
 * the detection, classification, OCR, and unified analyze scans. Each call site
 * supplies the api functions and completion behaviour; the timer lifecycle and
 * status state live here.
 */
export function createScan(opts: ScanOptions) {
	let status = $state<ScanStatus | null>(null);
	let timer: ReturnType<typeof setInterval> | null = null;

	function stopTimer() {
		if (timer) {
			clearInterval(timer);
			timer = null;
		}
	}

	function startPolling(rootId: string, path: string) {
		stopTimer();
		timer = setInterval(async () => {
			try {
				status = await opts.poll();
				if (!status.running) {
					stopTimer();
					opts.onToast(opts.doneMsg(status), 'success');
					opts.onComplete(rootId, path);
				}
			} catch {
				stopTimer();
			}
		}, opts.pollMs ?? 1000);
	}

	async function run(rootId: string, path: string) {
		try {
			status = await opts.start(rootId, path);
			opts.onToast(opts.startMsg(status), 'success');
			startPolling(rootId, path);
		} catch (e) {
			opts.onToast(e instanceof Error ? e.message : opts.errMsg, 'error');
		}
	}

	function dispose() {
		stopTimer();
	}

	return {
		get status() {
			return status;
		},
		get running() {
			return status?.running === true;
		},
		run,
		dispose,
	};
}

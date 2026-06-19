import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ScanStatus } from './api';
import { createScan } from './scans.svelte';

function status(over: Partial<ScanStatus> = {}): ScanStatus {
	return { running: true, total: 3, completed: 0, errors: 0, ...over };
}

describe('createScan', () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => vi.useRealTimers());

	it('starts, polls, and completes — firing toast + onComplete', async () => {
		const start = vi.fn(async () => status({ running: true, completed: 0 }));
		const poll = vi
			.fn<() => Promise<ScanStatus>>()
			.mockResolvedValueOnce(status({ running: true, completed: 2 }))
			.mockResolvedValueOnce(status({ running: false, completed: 3 }));
		const onToast = vi.fn();
		const onComplete = vi.fn();

		const scan = createScan({
			start,
			poll,
			startMsg: (s) => `start ${s.total}`,
			doneMsg: (s) => `done ${s.completed}`,
			errMsg: 'boom',
			onToast,
			onComplete,
		});

		await scan.run('root-0', 'photos');
		expect(start).toHaveBeenCalledWith('root-0', 'photos');
		expect(scan.running).toBe(true);
		expect(onToast).toHaveBeenCalledWith('start 3', 'success');

		// First poll tick — still running, no completion yet.
		await vi.advanceTimersByTimeAsync(1000);
		expect(scan.running).toBe(true);
		expect(onComplete).not.toHaveBeenCalled();

		// Second poll tick — finished.
		await vi.advanceTimersByTimeAsync(1000);
		expect(scan.running).toBe(false);
		expect(onToast).toHaveBeenCalledWith('done 3', 'success');
		expect(onComplete).toHaveBeenCalledWith('root-0', 'photos');

		// No further polling after completion.
		const pollCount = poll.mock.calls.length;
		await vi.advanceTimersByTimeAsync(3000);
		expect(poll).toHaveBeenCalledTimes(pollCount);
	});

	it('toasts an error when start fails', async () => {
		const onToast = vi.fn();
		const scan = createScan({
			start: vi.fn(async () => {
				throw new Error('nope');
			}),
			poll: vi.fn(),
			startMsg: () => 'start',
			doneMsg: () => 'done',
			errMsg: 'fallback',
			onToast,
			onComplete: vi.fn(),
		});

		await scan.run('root-0', '');
		expect(onToast).toHaveBeenCalledWith('nope', 'error');
		expect(scan.running).toBe(false);
	});

	it('dispose clears the pending poll timer', async () => {
		const poll = vi.fn(async () => status({ running: true }));
		const onComplete = vi.fn();
		const scan = createScan({
			start: vi.fn(async () => status()),
			poll,
			startMsg: () => 's',
			doneMsg: () => 'd',
			errMsg: 'e',
			onToast: vi.fn(),
			onComplete,
		});

		await scan.run('root-0', '');
		scan.dispose();
		await vi.advanceTimersByTimeAsync(5000);
		expect(poll).not.toHaveBeenCalled();
	});
});

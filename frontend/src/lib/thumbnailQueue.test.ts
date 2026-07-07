import { describe, expect, it } from 'vitest';
import { thumbnailQueue } from './thumbnailQueue';

/**
 * The exported `thumbnailQueue` is a process-wide singleton, so each test
 * appends to the same instance. We use small delays and a high concurrency
 * cap by enqueuing all items synchronously — the queue releases items in
 * order, so the run order is deterministic regardless of timing.
 */

function makeRunner(log: string[], label: string) {
	return () =>
		new Promise<void>((resolve) => {
			log.push(`start:${label}`);
			// Microtask delay so the queue's `finally` pumps the next item.
			setTimeout(() => {
				log.push(`end:${label}`);
				resolve();
			}, 0);
		});
}

describe('thumbnailQueue', () => {
	it('runs lower-priority items first when many are pending', async () => {
		// Fill the queue synchronously so the first 6 (cap) are released, but
		// we use priorities such that the lowest-value items should run first.
		// Since the cap is 6 and we enqueue 3, all 3 are released in priority
		// order on the first pump.
		const log: string[] = [];
		thumbnailQueue.enqueue(30, makeRunner(log, 'c'));
		thumbnailQueue.enqueue(10, makeRunner(log, 'a'));
		thumbnailQueue.enqueue(20, makeRunner(log, 'b'));
		// Wait for the runners to complete (two setTimeout(0) cycles).
		await new Promise((r) => setTimeout(r, 50));
		expect(log).toEqual([
			'start:a',
			'start:b',
			'start:c',
			'end:a',
			'end:b',
			'end:c',
		]);
	});

	it('cancel removes a pending entry before it starts', async () => {
		// Block all 6 slots with never-resolving runners, so subsequent
		// enqueues stay pending and cancellable.
		const blockers: Array<() => void> = [];
		for (let i = 0; i < 6; i++) {
			thumbnailQueue.enqueue(
				0,
				() => new Promise<void>((r) => blockers.push(r)),
			);
		}
		// Wait a tick so the pump has run.
		await new Promise((r) => setTimeout(r, 0));

		let ran = false;
		const cancel = thumbnailQueue.enqueue(0, () => {
			ran = true;
			return Promise.resolve();
		});
		cancel();

		// Release the blockers; the cancelled item must not run.
		for (const r of blockers) r();
		await new Promise((r) => setTimeout(r, 50));
		expect(ran).toBe(false);
	});

	it('cancel is a no-op once the runner has started', async () => {
		let ran = false;
		const cancel = thumbnailQueue.enqueue(0, () => {
			ran = true;
			return Promise.resolve();
		});
		// Let the runner complete.
		await new Promise((r) => setTimeout(r, 50));
		cancel();
		expect(ran).toBe(true);
	});
});

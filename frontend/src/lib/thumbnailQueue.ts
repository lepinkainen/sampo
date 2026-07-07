/**
 * Process-wide priority queue for thumbnail fetches.
 *
 * The browser caps concurrent requests per host (~6 in Chrome), and each
 * `GET /api/thumb/...` runs synchronously server-side (image decode or ffmpeg
 * for videos). Without ordering, a folder's requests fire in render order,
 * so a few large videos can claim HTTP slots for seconds while small images
 * pile up behind them.
 *
 * This queue mediates: callers add work with a numeric priority (lower =
 * sooner, see thumbnailPriority), and the queue releases at most
 * `maxConcurrent` runners at a time, always picking the lowest-priority
 * pending item next. Cached thumbnails return near-instantly from the
 * server, so reordering by cost does not delay them meaningfully.
 */

interface QueueItem {
	readonly priority: number;
	readonly run: () => Promise<void>;
}

const maxConcurrent = 6;

class ThumbnailQueue {
	#pending: QueueItem[] = [];
	#inFlight = 0;
	#pumpScheduled = false;

	/**
	 * Schedule `run` for execution. Returns a cancel function: calling it
	 * removes the item if it has not started yet. Once `run` has begun the
	 * cancel is a no-op (abort the in-flight work via the loader's own
	 * AbortController instead).
	 */
	enqueue(priority: number, run: () => Promise<void>): () => void {
		const item: QueueItem = { priority, run };
		this.#pending.push(item);
		this.#schedulePump();
		return () => {
			const idx = this.#pending.indexOf(item);
			if (idx !== -1) {
				this.#pending.splice(idx, 1);
			}
		};
	}

	/**
	 * Defer the pump by one microtask so a synchronous burst of enqueues
	 * (e.g. many ThumbnailCards becoming visible in the same frame) is
	 * considered together. Without this, the first `maxConcurrent` callers
	 * would start in enqueue order regardless of priority — defeating the
	 * point of prioritization when a folder opens with mixed media.
	 */
	#schedulePump() {
		if (this.#pumpScheduled) {
			return;
		}
		this.#pumpScheduled = true;
		queueMicrotask(() => {
			this.#pumpScheduled = false;
			this.#pump();
		});
	}

	#pump() {
		while (this.#inFlight < maxConcurrent && this.#pending.length > 0) {
			// Lowest priority value = highest precedence = front of the line.
			// Linear min-finding is simpler than a heap and fine for typical
			// folder sizes (tens to low hundreds of pending thumbnails).
			let minIdx = 0;
			for (let i = 1; i < this.#pending.length; i++) {
				if (this.#pending[i].priority < this.#pending[minIdx].priority) {
					minIdx = i;
				}
			}
			const next = this.#pending.splice(minIdx, 1)[0];
			this.#inFlight++;
			void next.run().finally(() => {
				this.#inFlight--;
				this.#schedulePump();
			});
		}
	}
}

export const thumbnailQueue = new ThumbnailQueue();

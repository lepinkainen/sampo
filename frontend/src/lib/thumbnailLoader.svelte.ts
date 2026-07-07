import { thumbnailUrl } from '$lib/api';
import type { FileEntry } from '$lib/types';

export type ThumbState = 'idle' | 'loading' | 'ready' | 'error';

const slowLoadingDelayMs = 800;

/**
 * Identity key for a thumbnail request. When it changes the loader must be
 * reset so a stale in-flight fetch can't paint the wrong image.
 */
export function thumbnailKey(
	rootId: string,
	entry: Pick<FileEntry, 'path' | 'modTime' | 'size' | 'hasThumb'>,
): string {
	return `${rootId}:${entry.path}:${entry.modTime}:${entry.size}:${entry.hasThumb}`;
}

/**
 * Shared thumbnail fetch state machine used by ThumbnailCard and DetailsPanel:
 * abortable fetch with token-based stale-response rejection, blob object-URL
 * lifecycle, and a delayed "slow loading" spinner flag.
 */
export class ThumbnailLoader {
	state = $state<ThumbState>('idle');
	objectUrl = $state<string | null>(null);
	showSlowLoading = $state(false);

	#controller: AbortController | null = null;
	#slowTimer: ReturnType<typeof setTimeout> | null = null;
	#token = 0;

	/** Abort any in-flight fetch and release the object URL. */
	cleanup() {
		this.#token++;
		this.#controller?.abort();
		this.#controller = null;
		this.#clearSlowLoading();
		this.#revokeUrl();
	}

	/** Cleanup and move to the given initial state for a new thumbnail key. */
	reset(initialState: ThumbState) {
		this.cleanup();
		this.state = initialState;
	}

	/** Mark the current thumbnail broken (e.g. the blob <img> failed to render). */
	fail() {
		this.#clearSlowLoading();
		this.#revokeUrl();
		this.state = 'error';
	}

	async fetch(rootId: string, path: string) {
		if (this.#controller) {
			return;
		}

		const token = ++this.#token;
		const controller = new AbortController();
		this.#controller = controller;
		this.state = 'loading';
		this.#startSlowLoading();

		try {
			const res = await fetch(thumbnailUrl(rootId, path), {
				signal: controller.signal,
				cache: 'no-store',
			});
			if (token !== this.#token) return;

			if (!res.ok) {
				throw new Error(`thumbnail failed: ${res.status}`);
			}

			const blob = await res.blob();
			if (token !== this.#token) return;
			if (!blob.type.startsWith('image/')) {
				throw new Error(`unexpected thumbnail type: ${blob.type}`);
			}

			const nextUrl = URL.createObjectURL(blob);
			this.#clearSlowLoading();
			this.#revokeUrl();
			this.objectUrl = nextUrl;
			this.state = 'ready';
		} catch (err) {
			if (controller.signal.aborted || token !== this.#token) {
				return;
			}
			this.#clearSlowLoading();
			console.error('Thumbnail failed', err);
			this.state = 'error';
		} finally {
			if (this.#controller === controller) {
				this.#controller = null;
			}
		}
	}

	#startSlowLoading() {
		this.#clearSlowLoading();
		this.#slowTimer = setTimeout(() => {
			this.showSlowLoading = true;
			this.#slowTimer = null;
		}, slowLoadingDelayMs);
	}

	#clearSlowLoading() {
		if (this.#slowTimer) {
			clearTimeout(this.#slowTimer);
			this.#slowTimer = null;
		}
		this.showSlowLoading = false;
	}

	#revokeUrl() {
		if (this.objectUrl) {
			URL.revokeObjectURL(this.objectUrl);
			this.objectUrl = null;
		}
	}
}

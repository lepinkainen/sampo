<script lang="ts">
interface Props {
	size?: number;
	speed?: string;
	label?: string;
	class?: string;
}

let {
	size = 16,
	speed = '1s',
	label = 'Loading',
	class: klass = '',
}: Props = $props();
</script>

<span
	class="bl bl-dither {klass}"
	role="status"
	aria-label={label}
	style="--size: {size}px; --speed: {speed}"
>
	<i></i><i></i><i></i><i></i><i></i><i></i><i></i><i></i>
	<i></i><i></i><i></i><i></i><i></i><i></i><i></i><i></i>
</span>

<style>
	.bl {
		--size: 32px;
		--speed: 1s;
		--bl-ease: cubic-bezier(0.77, 0, 0.175, 1);
		display: inline-flex;
		align-items: center;
		justify-content: center;
		color: inherit;
	}
	@keyframes bl-rotate {
		to {
			transform: rotate(1turn);
		}
	}
	@keyframes bl-fade {
		0%,
		100% {
			opacity: 1;
		}
		50% {
			opacity: 0.4;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.bl * {
			animation: none !important;
		}
		.bl {
			animation: bl-fade 1.4s var(--bl-ease) infinite;
		}
	}

	.bl-dither {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: max(1px, calc(var(--size) * 0.05));
		width: var(--size);
	}
	.bl-dither i {
		aspect-ratio: 1;
		background: currentColor;
		animation: bl-dith var(--speed) var(--bl-ease) infinite;
	}
	.bl-dither i:nth-child(1) {
		animation-delay: 0s;
	}
	.bl-dither i:nth-child(2) {
		animation-delay: calc(var(--speed) * 0.5);
	}
	.bl-dither i:nth-child(3) {
		animation-delay: calc(var(--speed) * 0.125);
	}
	.bl-dither i:nth-child(4) {
		animation-delay: calc(var(--speed) * 0.625);
	}
	.bl-dither i:nth-child(5) {
		animation-delay: calc(var(--speed) * 0.75);
	}
	.bl-dither i:nth-child(6) {
		animation-delay: calc(var(--speed) * 0.25);
	}
	.bl-dither i:nth-child(7) {
		animation-delay: calc(var(--speed) * 0.875);
	}
	.bl-dither i:nth-child(8) {
		animation-delay: calc(var(--speed) * 0.375);
	}
	.bl-dither i:nth-child(9) {
		animation-delay: calc(var(--speed) * 0.1875);
	}
	.bl-dither i:nth-child(10) {
		animation-delay: calc(var(--speed) * 0.6875);
	}
	.bl-dither i:nth-child(11) {
		animation-delay: calc(var(--speed) * 0.0625);
	}
	.bl-dither i:nth-child(12) {
		animation-delay: calc(var(--speed) * 0.5625);
	}
	.bl-dither i:nth-child(13) {
		animation-delay: calc(var(--speed) * 0.9375);
	}
	.bl-dither i:nth-child(14) {
		animation-delay: calc(var(--speed) * 0.4375);
	}
	.bl-dither i:nth-child(15) {
		animation-delay: calc(var(--speed) * 0.8125);
	}
	.bl-dither i:nth-child(16) {
		animation-delay: calc(var(--speed) * 0.3125);
	}
	@keyframes bl-dith {
		0%,
		100% {
			opacity: 0.1;
		}
		50% {
			opacity: 1;
		}
	}
</style>

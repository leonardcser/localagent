<div class="flex items-center gap-1 self-start px-1 py-2">
	<span class="dot size-1.5 rounded-full bg-text-muted"></span>
	<span class="dot size-1.5 rounded-full bg-text-muted"></span>
	<span class="dot size-1.5 rounded-full bg-text-muted"></span>
</div>

<style>
	.dot {
		animation: bounce 1.4s infinite ease-in-out both;
	}

	.dot:nth-child(1) {
		animation-delay: -0.32s;
	}

	.dot:nth-child(2) {
		animation-delay: -0.16s;
	}

	@keyframes bounce {
		0%,
		80%,
		100% {
			transform: scale(0.5);
			opacity: 0.3;
		}
		40% {
			transform: scale(1);
			opacity: 0.8;
		}
	}
</style>

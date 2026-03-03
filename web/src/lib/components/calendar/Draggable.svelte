<script lang="ts">
import { cn } from "$lib/utils";

interface Bounds {
	top: number;
	left: number;
	right: number;
	bottom: number;
}

type Grid = [number, number];

interface Props {
	class?: string;
	style?: string;
	bounds?: Bounds | null;
	grid?: Grid | null;
	disabled?: boolean;
	onDragEnd?: ((data: { x: number; y: number }) => void) | null;
	onclick?: (() => void) | null;
	children: import("svelte").Snippet;
}

export function reset() {
	top = 0;
	left = 0;
}

const DRAG_THRESHOLD = 4;

let {
	class: className = "",
	style = "",
	bounds = null,
	grid = null,
	disabled = false,
	onDragEnd = null,
	onclick = null,
	children,
}: Props = $props();

let isDragging = $state(false);
let isPending = $state(false);
let top = $state(0);
let left = $state(0);
let totalMovement = 0;

function clamp(val: number, min: number, max: number) {
	return Math.max(min, Math.min(val, max));
}

function snap(val: number, step: number) {
	return Math.round(val / step) * step;
}

let realTop = $derived.by(() => {
	let res = top;
	if (bounds) res = clamp(res, bounds.top, bounds.bottom);
	if (grid) res = snap(res, grid[1]);
	return res;
});

let realLeft = $derived.by(() => {
	let res = left;
	if (bounds) res = clamp(res, bounds.left, bounds.right);
	if (grid) res = snap(res, grid[0]);
	return res;
});

function handleMouseDown(event: MouseEvent) {
	if (disabled || event.button !== 0) return;
	isPending = true;
	totalMovement = 0;
}

function handleMouseMove(event: MouseEvent) {
	if (!isPending && !isDragging) return;

	if (isPending) {
		totalMovement += Math.abs(event.movementX) + Math.abs(event.movementY);
		if (totalMovement >= DRAG_THRESHOLD) {
			isPending = false;
			isDragging = true;
			document.body.classList.add("body-dragging");
		}
		return;
	}

	left += event.movementX;
	top += event.movementY;
}

function handleMouseUp() {
	if (isPending) {
		// No significant movement — treat as click
		isPending = false;
		onclick?.();
		return;
	}
	if (!isDragging) return;
	isDragging = false;
	document.body.classList.remove("body-dragging");
	if (bounds) {
		top = clamp(top, bounds.top, bounds.bottom);
		left = clamp(left, bounds.left, bounds.right);
	}
	if (grid) {
		top = snap(top, grid[1]);
		left = snap(left, grid[0]);
	}
	onDragEnd?.({ x: left, y: top });
}
</script>

{#if disabled}
	<div class={className} {style}>
		{@render children()}
	</div>
{:else}
	<button
		class={cn(
			className,
			"w-full cursor-pointer",
			isDragging && "z-10 cursor-grabbing",
		)}
		style="transform: translate({realLeft}px, {realTop}px); {style}"
		onmousedown={handleMouseDown}
	>
		{@render children()}
	</button>
{/if}

<svelte:window onmouseup={handleMouseUp} onmousemove={handleMouseMove} />

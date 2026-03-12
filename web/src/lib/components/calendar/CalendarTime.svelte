<script lang="ts">
import { onMount } from "svelte";
import { isSameDay, addDays } from "$lib/calendar";

interface Props {
  rowHeight: number;
  yOffset: number;
  viewStart: Date;
  numCols: number;
}

let { rowHeight, yOffset, viewStart, numCols }: Props = $props();

let now = $state(new Date());

let top = $derived(
  yOffset +
    rowHeight * now.getHours() +
    (rowHeight / 60) * now.getMinutes() +
    1,
);

let dayIndex = $derived.by(() => {
  for (let i = 0; i < numCols; i++) {
    if (isSameDay(now, addDays(viewStart, i))) return i;
  }
  return -1;
});

onMount(() => {
  const interval = setInterval(() => {
    now = new Date();
  }, 60000);
  return () => clearInterval(interval);
});

function formatTime(d: Date): string {
  return `${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`;
}
</script>

{#if dayIndex >= 0}
	<div
		class="pointer-events-none absolute left-0 z-20 flex w-full items-center"
		style="top: {top}px; height: 1.5px; background: var(--color-error)"
	>
		<div
			class="absolute rounded-full"
			style="left: calc({dayIndex} * 100% / {numCols} - 4px); width: 8px; height: 8px; background: var(--color-error)"
		></div>
		<div
			class="absolute rounded-sm px-1 text-[10px] font-medium backdrop-blur-sm"
			style="left: -48px; color: var(--color-error)"
		>
			{formatTime(now)}
		</div>
	</div>
{/if}

<script lang="ts">
import type { EventWithOverlap } from "$lib/calendar";
import { dayToIndex, addDays, isSameDay } from "$lib/calendar";
import Draggable from "./Draggable.svelte";

interface Props {
  event: EventWithOverlap;
  calendarWidth: number;
  rowHeight: number;
  yOffset: number;
  weekStart: Date;
  onDragEnd?: (delta: { x: number; y: number }) => void;
}

let { event, calendarWidth, rowHeight, yOffset, weekStart, onDragEnd }: Props =
  $props();

let start = $derived(new Date(event.startMs));
let end = $derived(new Date(event.endMs));
let durationMin = $derived((event.endMs - event.startMs) / 60000);

let colIndex = $derived.by(() => {
  for (let i = 0; i < 7; i++) {
    if (isSameDay(start, addDays(weekStart, i))) return i;
  }
  return 0;
});

let top = $derived(
  start.getHours() * rowHeight + (start.getMinutes() / 60) * rowHeight,
);
let left = $derived((100 / event.overlapCount) * event.overlapIndex);
let height = $derived((durationMin / 60) * rowHeight);
let width = $derived(100 / event.overlapCount);

function formatTime(d: Date): string {
  return d.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

let draggableRef = $state<{ reset: () => void }>();

$effect(() => {
  if (draggableRef && event.draggable) {
    draggableRef.reset();
  }
});
</script>

<div
	class="absolute"
	style="top: {top + yOffset}px; left: {left}%; height: {height}px; width: {width}%; grid-column-start: {colIndex + 1}; grid-column-end: {colIndex + 2}"
>
	<Draggable
		bind:this={draggableRef}
		class="relative h-full w-full"
		bounds={{
			top: -top,
			bottom: rowHeight * 24 - top - (durationMin / 60) * rowHeight,
			left: -(colIndex * calendarWidth) / 7,
			right: calendarWidth - ((colIndex + 1) * calendarWidth) / 7,
		}}
		grid={[calendarWidth / 7, rowHeight / 4]}
		disabled={!event.draggable}
		onDragEnd={onDragEnd ?? null}
	>
		<div
			class="h-full rounded-sm border-l-2 px-1.5 py-0.5 text-[11px] backdrop-blur-sm"
			style="background-color: color-mix(in srgb, {event.color} 15%, transparent); border-color: {event.color}; color: {event.color}"
		>
			{#if durationMin >= 60}
				<div class="truncate opacity-70">{formatTime(start)}</div>
			{/if}
			<div class="truncate font-semibold">{event.title}</div>
			{#if event.note && durationMin >= 45}
				<div class="truncate opacity-70">{event.note}</div>
			{/if}
		</div>
	</Draggable>
</div>

<script lang="ts">
import {
  computeCalendarEventsOverlaps,
  calculateNewEventTime,
  addDays,
  isSameDay,
  type CalendarEvent as CalendarEventType,
} from "$lib/calendar";
import { slotStore } from "$lib/stores/slot.svelte";
import { onMount } from "svelte";
import CalendarTime from "./CalendarTime.svelte";
import CalendarEventComp from "./CalendarEvent.svelte";
import CalendarDayEvent from "./CalendarDayEvent.svelte";

interface Props {
  events: CalendarEventType[];
  indexColWidth: number;
  rowHeight: number;
  weekStart: Date;
}

let { events, indexColWidth, rowHeight, weekStart }: Props = $props();

let timedEvents = $derived(events.filter((e) => !e.isAllDay));
let dayEvents = $derived(events.filter((e) => e.isAllDay));

let eventsWithOverlap = $derived(computeCalendarEventsOverlaps(timedEvents));

let dayEventsByCol = $derived.by(() => {
  const byCol: CalendarEventType[][] = Array.from({ length: 7 }, () => []);
  for (const evt of dayEvents) {
    const d = new Date(evt.startMs);
    for (let i = 0; i < 7; i++) {
      if (isSameDay(d, addDays(weekStart, i))) {
        byCol[i].push(evt);
        break;
      }
    }
  }
  return byCol;
});

let scrollableRef = $state<HTMLDivElement>();
let calendarBodyRef = $state<HTMLDivElement>();
let calendarWidth = $state(0);

onMount(() => {
  if (scrollableRef) {
    const saved = localStorage.getItem("calendarScrollY");
    if (saved) {
      scrollableRef.scrollTo(0, parseInt(saved));
    } else {
      scrollableRef.scrollTo(0, new Date().getHours() * rowHeight);
    }
  }

  if (calendarBodyRef) {
    calendarWidth = calendarBodyRef.clientWidth;
    const ro = new ResizeObserver(() => {
      if (calendarBodyRef) calendarWidth = calendarBodyRef.clientWidth;
    });
    ro.observe(calendarBodyRef);
    return () => ro.disconnect();
  }
});

function handleScroll() {
  if (scrollableRef) {
    localStorage.setItem("calendarScrollY", String(scrollableRef.scrollTop));
  }
}

function handleDragEnd(
  event: CalendarEventType & { overlapIndex: number; overlapCount: number },
  delta: { x: number; y: number },
) {
  if (!event.slotId) return;
  const newTime = calculateNewEventTime(event, delta, calendarWidth, rowHeight);
  slotStore.update(event.slotId, {
    startAtMs: newTime.startMs,
    endAtMs: newTime.endMs,
  });
}

function formatHour(hour: number): string {
  return `${hour.toString().padStart(2, "0")}:00`;
}

let rowStartOffset = $derived(rowHeight / 2);
</script>

<!-- All-day events -->
{#if dayEvents.length > 0}
	<div
		class="grid auto-rows-min grid-cols-7 border-b border-border"
		style="padding-left: {indexColWidth}px"
	>
		{#each dayEventsByCol as col}
			<div class="min-h-[28px] border-r border-border/50 px-0.5 py-0.5">
				{#each col as evt}
					<CalendarDayEvent event={evt} />
				{/each}
			</div>
		{/each}
	</div>
{/if}

<!-- Scrollable time grid -->
<div
	class="flex flex-1 overflow-auto"
	bind:this={scrollableRef}
	onscroll={handleScroll}
>
	<div style="width: {indexColWidth}px">
		{#each Array.from({ length: 25 }) as _, hour}
			<div
				class="flex items-center justify-center text-[10px] text-text-muted"
				style="height: {rowHeight}px"
			>
				{formatHour(hour)}
			</div>
		{/each}
	</div>
	<div class="relative grid flex-1 grid-cols-7" bind:this={calendarBodyRef}>
		<CalendarTime {rowHeight} yOffset={rowStartOffset} {weekStart} />

		{#each eventsWithOverlap as event}
			<CalendarEventComp
				{event}
				{calendarWidth}
				{rowHeight}
				yOffset={rowStartOffset}
				{weekStart}
				onDragEnd={(d) => handleDragEnd(event, d)}
			/>
		{/each}

		<!-- Start padding -->
		{#each Array.from({ length: 7 }) as _}
			<div
				class="border-b border-r border-border/50"
				style="height: {rowStartOffset}px"
			></div>
		{/each}

		<!-- Hour cells -->
		{#each Array.from({ length: 24 }) as _, hour}
			{#each Array.from({ length: 7 }) as _}
				<div
					class="border-b border-r border-border/50"
					style="height: {rowHeight}px"
				></div>
			{/each}
		{/each}
	</div>
</div>

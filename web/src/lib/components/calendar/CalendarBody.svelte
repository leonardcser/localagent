<script lang="ts">
import {
  computeCalendarEventsOverlaps,
  calculateNewEventTime,
  addDays,
  isSameDay,
  type CalendarEvent as CalendarEventType,
} from "$lib/calendar";
import { blockStore } from "$lib/stores/block.svelte";
import { taskStore } from "$lib/stores/task.svelte";
import { goto } from "$app/navigation";
import { onMount } from "svelte";
import { Icon } from "svelte-icons-pack";
import { FiX } from "svelte-icons-pack/fi";
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
  if (!event.blockId) return;
  const newTime = calculateNewEventTime(event, delta, calendarWidth, rowHeight);
  blockStore.update(event.blockId, {
    startAtMs: newTime.startMs,
    endAtMs: newTime.endMs,
  });
}

function formatHour(hour: number): string {
  return `${hour.toString().padStart(2, "0")}:00`;
}

let rowStartOffset = $derived(rowHeight / 2);

// --- Click-to-create ---

interface CreateState {
  startMs: number;
  endMs: number;
  taskId: string;
  note: string;
}

let createState = $state<CreateState | null>(null);

function msToTimeStr(ms: number): string {
  const d = new Date(ms);
  return `${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`;
}

function handleGridClick(e: MouseEvent) {
  if (!calendarBodyRef || !calendarWidth) return;
  // Ignore clicks that originated on event blocks
  const target = e.target as HTMLElement;
  if (target.closest("[data-calendar-event]")) return;

  const rect = calendarBodyRef.getBoundingClientRect();
  const x = e.clientX - rect.left;
  const y = e.clientY - rect.top - rowStartOffset;
  if (y < 0) return;

  const colIndex = Math.floor((x / calendarWidth) * 7);
  const totalMinutes = Math.floor(((y / rowHeight) * 60) / 15) * 15;
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;

  const day = addDays(weekStart, colIndex);
  day.setHours(hours, minutes, 0, 0);
  const startMs = day.getTime();
  const endMs = startMs + 60 * 60 * 1000; // default 1h

  createState = { startMs, endMs, taskId: "", note: "" };
}

async function submitCreate() {
  if (!createState || !createState.taskId) return;
  await blockStore.add({
    taskId: createState.taskId,
    startAtMs: createState.startMs,
    endAtMs: createState.endMs,
    note: createState.note || undefined,
  });
  createState = null;
}

let activeTasks = $derived(taskStore.tasks.filter((t) => t.status !== "done"));
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
	<div
		class="relative grid flex-1 grid-cols-7 cursor-crosshair"
		role="presentation"
		bind:this={calendarBodyRef}
		onclick={handleGridClick}
		onkeydown={() => {}}
	>
		<CalendarTime {rowHeight} yOffset={rowStartOffset} {weekStart} />

		{#each eventsWithOverlap as event}
			<CalendarEventComp
				{event}
				{calendarWidth}
				{rowHeight}
				yOffset={rowStartOffset}
				{weekStart}
				onDragEnd={(d) => handleDragEnd(event, d)}
				onDelete={event.blockId ? () => blockStore.remove(event.blockId!) : undefined}
				onViewTask={(taskId) => goto(`/tasks?select=${taskId}`)}
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

<!-- Create block modal -->
{#if createState}
	<div
		class="fixed inset-0 z-40 bg-black/20 backdrop-blur-[1px]"
		role="presentation"
		onclick={() => (createState = null)}
		onkeydown={() => {}}
	></div>
	<div
		class="fixed left-1/2 top-1/2 z-50 w-72 -translate-x-1/2 -translate-y-1/2 rounded-xl border border-border bg-bg-secondary p-4 shadow-elevated"
	>
		<div class="mb-3 flex items-center justify-between">
			<span class="text-[13px] font-semibold text-text-primary">Add time block</span>
			<button onclick={() => (createState = null)} class="text-text-muted hover:text-text-secondary">
				<Icon src={FiX} size="14" />
			</button>
		</div>

		<div class="mb-3 rounded-lg bg-bg-tertiary px-3 py-2 text-[12px] text-text-secondary">
			{new Date(createState.startMs).toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric" })}
			<span class="mx-1 text-text-muted">·</span>
			<input
				type="time"
				value={msToTimeStr(createState.startMs)}
				onchange={(e) => {
					if (!createState) return;
					const [h, m] = (e.target as HTMLInputElement).value.split(":").map(Number);
					const d = new Date(createState.startMs);
					d.setHours(h, m, 0, 0);
					const dur = createState.endMs - createState.startMs;
					createState.startMs = d.getTime();
					createState.endMs = d.getTime() + dur;
				}}
				class="rounded border border-border bg-transparent px-1 text-[12px] text-text-primary outline-none focus:border-accent"
			/>
			<span class="mx-1 text-text-muted">–</span>
			<input
				type="time"
				value={msToTimeStr(createState.endMs)}
				onchange={(e) => {
					if (!createState) return;
					const [h, m] = (e.target as HTMLInputElement).value.split(":").map(Number);
					const d = new Date(createState.endMs);
					d.setHours(h, m, 0, 0);
					createState.endMs = d.getTime();
				}}
				class="rounded border border-border bg-transparent px-1 text-[12px] text-text-primary outline-none focus:border-accent"
			/>
		</div>

		<div class="mb-3">
			<label for="create-block-task" class="mb-1 block text-[11px] text-text-muted">Task</label>
			<select
				id="create-block-task"
				bind:value={createState.taskId}
				class="w-full rounded-lg border border-border bg-bg-tertiary px-2 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
			>
				<option value="">Select a task…</option>
				{#each activeTasks as task}
					<option value={task.id}>{task.title}</option>
				{/each}
			</select>
		</div>

		<div class="mb-4">
			<label for="create-block-note" class="mb-1 block text-[11px] text-text-muted">Note (optional)</label>
			<input
				id="create-block-note"
				type="text"
				bind:value={createState.note}
				placeholder="e.g. deep work session"
				class="w-full rounded-lg border border-border bg-bg-tertiary px-2 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
			/>
		</div>

		<button
			onclick={submitCreate}
			disabled={!createState.taskId}
			class="w-full rounded-lg bg-accent py-1.5 text-[12px] font-medium text-white hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
		>
			Add block
		</button>
	</div>
{/if}

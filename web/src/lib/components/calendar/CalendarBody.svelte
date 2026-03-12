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
  viewStart: Date;
  numCols: number;
  onViewTask?: (taskId: string) => void;
}

let {
  events,
  indexColWidth,
  rowHeight,
  viewStart,
  numCols,
  onViewTask,
}: Props = $props();

let timedEvents = $derived(events.filter((e) => !e.isAllDay));
let dayEvents = $derived(events.filter((e) => e.isAllDay));

let eventsWithOverlap = $derived(computeCalendarEventsOverlaps(timedEvents));

let dayEventsByCol = $derived.by(() => {
  const byCol: CalendarEventType[][] = Array.from(
    { length: numCols },
    () => [],
  );
  for (const evt of dayEvents) {
    const d = new Date(evt.startMs);
    for (let i = 0; i < numCols; i++) {
      if (isSameDay(d, addDays(viewStart, i))) {
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

// --- Timed event drag ---
function handleDragEnd(
  event: CalendarEventType & { overlapIndex: number; overlapCount: number },
  delta: { x: number; y: number },
) {
  if (!event.blockId) return;
  const newTime = calculateNewEventTime(
    event,
    delta,
    calendarWidth,
    rowHeight,
    numCols,
    viewStart,
  );
  blockStore.update(event.blockId, {
    startAtMs: newTime.startMs,
    endAtMs: newTime.endMs,
  });
}

// --- Timed event resize ---
function handleResize(event: CalendarEventType, newEndMs: number) {
  if (!event.blockId) return;
  blockStore.update(event.blockId, { endAtMs: newEndMs });
}

// --- All-day event drag (task due date change) ---
function handleAllDayMove(
  event: CalendarEventType,
  colI: number,
  colDelta: number,
) {
  const newCol = Math.max(0, Math.min(numCols - 1, colI + colDelta));
  const newDate = addDays(viewStart, newCol);
  const dateStr = newDate.toISOString().slice(0, 10);
  taskStore.update(event.taskId, { due: dateStr });
}

function formatHour(hour: number): string {
  return `${hour.toString().padStart(2, "0")}:00`;
}

let rowStartOffset = $derived(rowHeight / 2);
let colWidth = $derived(calendarWidth / numCols);

// --- Click-and-drag to create ---

interface CreateState {
  startMs: number;
  endMs: number;
  taskId: string;
  note: string;
}

let createState = $state<CreateState | null>(null);

interface DragPreview {
  col: number;
  anchorMinutes: number;
  currentMinutes: number;
}

let dragPreview = $state<DragPreview | null>(null);
const DRAG_THRESHOLD = 4;
let dragStartY = 0;
let didDrag = $state(false);

function yToMinutes(y: number): number {
  const raw = ((y - rowStartOffset) / rowHeight) * 60;
  return Math.max(0, Math.min(24 * 60, Math.floor(raw / 15) * 15));
}

function xToCol(x: number): number {
  return Math.max(
    0,
    Math.min(numCols - 1, Math.floor((x / calendarWidth) * numCols)),
  );
}

function handleGridMousedown(e: MouseEvent) {
  if (!calendarBodyRef || !calendarWidth) return;
  if (e.button !== 0) return;
  // Don't start drag if clicking on an existing event
  if ((e.target as HTMLElement).closest("[data-calendar-event]")) return;

  const rect = calendarBodyRef.getBoundingClientRect();
  const x = e.clientX - rect.left;
  const y = e.clientY - rect.top;
  if (y < rowStartOffset) return;

  const col = xToCol(x);
  const minutes = yToMinutes(y);

  dragStartY = e.clientY;
  didDrag = false;
  dragPreview = { col, anchorMinutes: minutes, currentMinutes: minutes };

  window.addEventListener("mousemove", handleGridMousemove);
  window.addEventListener("mouseup", handleGridMouseup);
}

function handleGridMousemove(e: MouseEvent) {
  if (!dragPreview || !calendarBodyRef) return;

  if (!didDrag && Math.abs(e.clientY - dragStartY) > DRAG_THRESHOLD) {
    didDrag = true;
  }

  const rect = calendarBodyRef.getBoundingClientRect();
  const y = e.clientY - rect.top;
  dragPreview.currentMinutes = yToMinutes(y);
}

function handleGridMouseup() {
  window.removeEventListener("mousemove", handleGridMousemove);
  window.removeEventListener("mouseup", handleGridMouseup);

  if (!dragPreview) return;

  let startMin = Math.min(
    dragPreview.anchorMinutes,
    dragPreview.currentMinutes,
  );
  let endMin = Math.max(dragPreview.anchorMinutes, dragPreview.currentMinutes);

  // If no drag (just a click), default to 1-hour block
  if (!didDrag || endMin - startMin < 15) {
    endMin = Math.min(startMin + 60, 24 * 60);
  }

  const day = addDays(viewStart, dragPreview.col);
  const startDay = new Date(day);
  startDay.setHours(Math.floor(startMin / 60), startMin % 60, 0, 0);
  const endDay = new Date(day);
  endDay.setHours(Math.floor(endMin / 60), endMin % 60, 0, 0);

  dragPreview = null;
  createTaskSearch = "";
  createTaskDropdownOpen = false;
  createState = {
    startMs: startDay.getTime(),
    endMs: endDay.getTime(),
    taskId: "",
    note: "",
  };
}

function msToTimeStr(ms: number): string {
  const d = new Date(ms);
  return `${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`;
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

let createTaskSearch = $state("");
let createTaskDropdownOpen = $state(false);

let filteredTasks = $derived(
  createTaskSearch
    ? activeTasks.filter((t) =>
        t.title.toLowerCase().includes(createTaskSearch.toLowerCase()),
      )
    : activeTasks,
);

function selectTask(task: { id: string; title: string }) {
  if (!createState) return;
  createState.taskId = task.id;
  createTaskSearch = task.title;
  createTaskDropdownOpen = false;
}

let todayDate = $state(new Date());
</script>

<svelte:window
	onkeydown={(e) => {
		if (e.key === "Escape" && createState) {
			createState = null;
			createTaskSearch = "";
			createTaskDropdownOpen = false;
		}
	}}
/>

<div class="flex min-h-0 flex-1 flex-col">
  <!-- All-day events row -->
  {#if dayEvents.length > 0}
    <div
      class="grid shrink-0 auto-rows-min border-b border-border"
      style="padding-left: {indexColWidth}px; grid-template-columns: repeat({numCols}, 1fr)"
    >
      {#each dayEventsByCol as col, colI}
        <div class="min-h-[28px] border-r border-border/50 px-0.5 py-0.5">
          {#each col as evt}
            <CalendarDayEvent
              event={evt}
              {colWidth}
              onMove={(delta) => handleAllDayMove(evt, colI, delta)}
              onViewTask={onViewTask}
            />
          {/each}
        </div>
      {/each}
    </div>
  {/if}

  <!-- Scrollable time grid -->
  <div
    class="flex min-h-0 flex-1 items-start overflow-auto"
    bind:this={scrollableRef}
    onscroll={handleScroll}
  >
    <!-- Hour labels -->
    <div class="shrink-0" style="width: {indexColWidth}px">
      {#each Array.from({ length: 25 }) as _, hour}
        <div
          class="flex items-center justify-center text-[10px] text-text-muted"
          style="height: {rowHeight}px"
        >
          {formatHour(hour)}
        </div>
      {/each}
    </div>

    <!-- Grid -->
    <div
      class="relative grid flex-1 cursor-crosshair select-none"
      role="presentation"
      style="grid-template-columns: repeat({numCols}, 1fr)"
      bind:this={calendarBodyRef}
      onmousedown={handleGridMousedown}
      onkeydown={() => {}}
    >
      <CalendarTime {rowHeight} yOffset={rowStartOffset} {viewStart} {numCols} />

      {#each eventsWithOverlap as event}
        <CalendarEventComp
          {event}
          {calendarWidth}
          {rowHeight}
          yOffset={rowStartOffset}
          {viewStart}
          {numCols}
          onDragEnd={(d) => handleDragEnd(event, d)}
          onResize={(newEnd) => handleResize(event, newEnd)}
          onDelete={event.blockId ? () => blockStore.remove(event.blockId!) : undefined}
          onViewTask={onViewTask}
        />
      {/each}

      <!-- Drag preview block -->
      {#if dragPreview && didDrag}
        {@const previewStartMin = Math.min(dragPreview.anchorMinutes, dragPreview.currentMinutes)}
        {@const previewEndMin = Math.max(dragPreview.anchorMinutes, dragPreview.currentMinutes)}
        {@const topPx = rowStartOffset + (previewStartMin / 60) * rowHeight}
        {@const heightPx = Math.max(rowHeight / 4, ((previewEndMin - previewStartMin) / 60) * rowHeight)}
        <div
          class="pointer-events-none absolute z-20 rounded-md border border-accent/60 bg-accent/20"
          style="
            left: calc({dragPreview.col} * (100% / {numCols}) + 2px);
            width: calc(100% / {numCols} - 4px);
            top: {topPx}px;
            height: {heightPx}px;
          "
        >
          <span class="block px-1.5 pt-0.5 text-[10px] font-medium text-accent">
            {Math.floor(previewStartMin / 60).toString().padStart(2, "0")}:{(previewStartMin % 60).toString().padStart(2, "0")}
            –
            {Math.floor(previewEndMin / 60).toString().padStart(2, "0")}:{(previewEndMin % 60).toString().padStart(2, "0")}
          </span>
        </div>
      {/if}

      <!-- Half-row top padding -->
      {#each Array.from({ length: numCols }) as _, colIdx}
        <div
          class="border-b border-r border-border/50 {isSameDay(addDays(viewStart, colIdx), todayDate) ? 'bg-accent/[0.03]' : ''}"
          style="height: {rowStartOffset}px"
        ></div>
      {/each}

      <!-- Hour cells -->
      {#each Array.from({ length: 24 }) as _}
        {#each Array.from({ length: numCols }) as _, colIdx}
          <div class="border-b border-r border-border/50 {isSameDay(addDays(viewStart, colIdx), todayDate) ? 'bg-accent/[0.03]' : ''}" style="height: {rowHeight}px"></div>
        {/each}
      {/each}
    </div>
  </div>
</div>

<!-- Create block modal -->
{#if createState}
  <div
    class="fixed inset-0 z-40 bg-black/20 backdrop-blur-[1px]"
    role="presentation"
    onclick={() => { createState = null; createTaskSearch = ""; createTaskDropdownOpen = false; }}
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
      {new Date(createState.startMs).toLocaleDateString(undefined, {
        weekday: "short",
        month: "short",
        day: "numeric",
      })}
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
      <div class="relative">
        <input
          id="create-block-task"
          type="text"
          bind:value={createTaskSearch}
          placeholder="Search tasks…"
          autocomplete="off"
          onfocus={() => (createTaskDropdownOpen = true)}
          oninput={() => {
            if (createState) createState.taskId = "";
            createTaskDropdownOpen = true;
          }}
          class="w-full rounded-lg border border-border bg-bg-tertiary px-2 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
        />
        {#if createTaskDropdownOpen && filteredTasks.length > 0}
          <div class="absolute left-0 right-0 top-full z-10 mt-1 max-h-48 overflow-y-auto rounded-lg border border-border bg-bg-secondary shadow-elevated">
            {#each filteredTasks as task}
              <button
                type="button"
                class="flex w-full items-start gap-2 px-2 py-1.5 text-left hover:bg-overlay-light"
                onmousedown={(e) => { e.preventDefault(); selectTask(task); }}
              >
                <div class="min-w-0 flex-1">
                  <div class="truncate text-[12px] text-text-primary">{task.title}</div>
                  <div class="flex items-center gap-1.5 text-[10px] text-text-muted">
                    {#if task.priority}
                      <span class="capitalize {task.priority === 'high' ? 'text-error' : task.priority === 'medium' ? 'text-warning' : 'text-accent'}">{task.priority}</span>
                    {/if}
                    {#if task.due}
                      <span>{task.due}</span>
                    {/if}
                    {#if task.tags?.length}
                      <span class="truncate">{task.tags.join(', ')}</span>
                    {/if}
                  </div>
                </div>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    </div>

    <div class="mb-4">
      <label for="create-block-note" class="mb-1 block text-[11px] text-text-muted"
        >Note (optional)</label
      >
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

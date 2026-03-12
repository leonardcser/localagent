<script lang="ts">
import { onMount } from "svelte";
import { blockStore } from "$lib/stores/block.svelte";
import { taskStore } from "$lib/stores/task.svelte";
import {
  getWeekStart,
  addDays,
  blockToEvent,
  taskToEvent,
  type CalendarEvent,
  type CalendarView,
} from "$lib/calendar";
import CalendarHeader from "./CalendarHeader.svelte";
import CalendarBody from "./CalendarBody.svelte";
import TaskDetailPanel from "$lib/components/TaskDetailPanel.svelte";
import type { Task } from "$lib/api";

interface Props {
  indexColWidth?: number;
  rowHeight?: number;
}

let { indexColWidth = 52, rowHeight = 48 }: Props = $props();

let view = $state<CalendarView>("week");
let currentDate = $state(new Date());

// Task detail panel
let detailTask = $state<Task | null>(null);
let detailPanelMode = $state<"add" | "edit">("edit");
let detailPanelOpen = $state(false);
let addParentId = $state("");

function openTaskDetail(taskId: string) {
  const task = taskStore.tasks.find((t) => t.id === taskId);
  if (task) {
    taskStore.selectedId = taskId;
    detailTask = task;
    detailPanelMode = "edit";
    detailPanelOpen = true;
  }
}

function openAddTask(parentId = "") {
  taskStore.selectedId = "";
  detailTask = null;
  detailPanelMode = "add";
  addParentId = parentId;
  detailPanelOpen = true;
}

function closeDetail() {
  detailPanelOpen = false;
  detailTask = null;
}

let numCols = $derived(view === "day" ? 1 : view === "3day" ? 3 : 7);

let viewStart = $derived.by(() => {
  const d = new Date(
    currentDate.getFullYear(),
    currentDate.getMonth(),
    currentDate.getDate(),
  );
  return view === "week" ? getWeekStart(d) : d;
});

let viewEnd = $derived(addDays(viewStart, numCols));

function navigate(dir: -1 | 1) {
  currentDate = addDays(currentDate, dir * numCols);
}

function goToToday() {
  currentDate = new Date();
}

onMount(() => {
  if (window.innerWidth < 640) view = "day";
  taskStore.load();
});

$effect(() => {
  blockStore.load(viewStart.getTime(), viewEnd.getTime());
});

let events = $derived.by(() => {
  const result: CalendarEvent[] = [];

  for (const block of blockStore.blocks) {
    const task = taskStore.tasks.find((t) => t.id === block.taskId);
    result.push(blockToEvent(block, task));
  }

  for (const task of taskStore.tasks) {
    const evt = taskToEvent(task);
    if (evt) {
      const due = new Date(evt.startMs);
      if (due >= viewStart && due < viewEnd) result.push(evt);
    }
  }

  return result;
});
</script>

<div class="flex h-full flex-col bg-bg">
  <CalendarHeader
    {indexColWidth}
    date={currentDate}
    {view}
    {numCols}
    {viewStart}
    {navigate}
    {goToToday}
    setView={(v) => (view = v)}
  />
  <div class="flex flex-1 overflow-hidden">
    <CalendarBody
      {events}
      {indexColWidth}
      {rowHeight}
      {viewStart}
      {numCols}
      onViewTask={openTaskDetail}
    />
    {#if detailPanelOpen}
      <TaskDetailPanel
        task={detailPanelMode === "edit" ? detailTask : null}
        parentId={addParentId}
        onClose={closeDetail}
        onSelectTask={(t) => openTaskDetail(t.id)}
        onAddSubtask={openAddTask}
      />
    {/if}
  </div>
</div>

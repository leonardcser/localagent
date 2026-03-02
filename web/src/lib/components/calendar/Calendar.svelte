<script lang="ts">
import { onMount } from "svelte";
import { slotStore } from "$lib/stores/slot.svelte";
import { taskStore } from "$lib/stores/task.svelte";
import {
  getWeekStart,
  addDays,
  slotToEvent,
  taskToEvent,
  type CalendarEvent,
} from "$lib/calendar";
import CalendarHeader from "./CalendarHeader.svelte";
import CalendarBody from "./CalendarBody.svelte";

interface Props {
  indexColWidth?: number;
  rowHeight?: number;
}

let { indexColWidth = 52, rowHeight = 48 }: Props = $props();

let currentDate = $state(new Date());
let weekStart = $derived(getWeekStart(currentDate));
let weekEnd = $derived(addDays(weekStart, 7));

function setDate(d: Date) {
  currentDate = d;
}

$effect(() => {
  slotStore.load(weekStart.getTime(), weekEnd.getTime());
});

onMount(() => {
  taskStore.load();
});

let events = $derived.by(() => {
  const result: CalendarEvent[] = [];

  for (const slot of slotStore.slots) {
    const task = taskStore.tasks.find((t) => t.id === slot.taskId);
    result.push(slotToEvent(slot, task));
  }

  for (const task of taskStore.tasks) {
    const evt = taskToEvent(task);
    if (evt) {
      const due = new Date(evt.startMs);
      if (due >= weekStart && due < weekEnd) {
        result.push(evt);
      }
    }
  }

  return result;
});
</script>

<div class="flex h-full flex-col bg-bg">
	<CalendarHeader {indexColWidth} date={currentDate} {setDate} />
	<CalendarBody {events} {indexColWidth} {rowHeight} {weekStart} />
</div>

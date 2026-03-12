<script lang="ts">
import type { EventWithOverlap } from "$lib/calendar";
import { addDays, isSameDay } from "$lib/calendar";
import { ContextMenu } from "bits-ui";
import { Icon } from "svelte-icons-pack";
import { FiExternalLink, FiTrash2 } from "svelte-icons-pack/fi";
import Draggable from "./Draggable.svelte";

interface Props {
  event: EventWithOverlap;
  calendarWidth: number;
  rowHeight: number;
  yOffset: number;
  viewStart: Date;
  numCols: number;
  onDragEnd?: (delta: { x: number; y: number }) => void;
  onResize?: (newEndMs: number) => void;
  onDelete?: () => void;
  onViewTask?: (taskId: string) => void;
}

let {
  event,
  calendarWidth,
  rowHeight,
  yOffset,
  viewStart,
  numCols,
  onDragEnd,
  onResize,
  onDelete,
  onViewTask,
}: Props = $props();

let start = $derived(new Date(event.startMs));
let durationMin = $derived((event.endMs - event.startMs) / 60000);

let colIndex = $derived.by(() => {
  for (let i = 0; i < numCols; i++) {
    if (isSameDay(start, addDays(viewStart, i))) return i;
  }
  return 0;
});

let colWidth = $derived(calendarWidth / numCols);
let top = $derived(
  start.getHours() * rowHeight + (start.getMinutes() / 60) * rowHeight,
);
// Position within the column, accounting for overlapping events
let colLeftPx = $derived(colIndex * colWidth);
let overlapWidth = $derived(colWidth / event.overlapCount);
let leftPx = $derived(colLeftPx + overlapWidth * event.overlapIndex);
// min height = 15 min
let baseHeight = $derived(
  Math.max((durationMin / 60) * rowHeight, rowHeight / 4),
);

// --- Resize ---
let resizing = $state(false);
let resizeDeltaY = $state(0);
let resizeStartY = 0;

function startResize(e: MouseEvent) {
  e.stopPropagation();
  e.preventDefault();
  resizing = true;
  resizeStartY = e.clientY;
  resizeDeltaY = 0;
}

function handleResizeMove(e: MouseEvent) {
  if (!resizing) return;
  resizeDeltaY = e.clientY - resizeStartY;
}

function handleResizeUp() {
  if (!resizing) return;
  resizing = false;
  // Snap to 15-min grid
  const step = rowHeight / 4;
  const snapped = Math.round(resizeDeltaY / step) * step;
  const deltaMs = (snapped / rowHeight) * 3600000;
  const newEndMs = Math.max(event.startMs + 15 * 60000, event.endMs + deltaMs);
  onResize?.(newEndMs);
  resizeDeltaY = 0;
}

let displayHeight = $derived(
  Math.max(baseHeight + resizeDeltaY, rowHeight / 4),
);

function formatTime(d: Date): string {
  return `${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`;
}

let draggableRef = $state<{ reset: () => void }>();

$effect(() => {
  if (draggableRef && event.draggable) draggableRef.reset();
});

// --- Click-to-view (bypasses ContextMenu.Trigger event interception) ---
const CLICK_THRESHOLD = 4;
let clickPending = $state(false);
let clickMovement = 0;

function handleEventMouseDown(e: MouseEvent) {
  if (e.button !== 0) return;
  clickPending = true;
  clickMovement = 0;
}

function handleEventMouseMove(e: MouseEvent) {
  if (!clickPending) return;
  clickMovement += Math.abs(e.movementX) + Math.abs(e.movementY);
  if (clickMovement >= CLICK_THRESHOLD) {
    clickPending = false;
  }
}

function handleEventMouseUp() {
  if (clickPending) {
    clickPending = false;
    onViewTask?.(event.taskId);
  }
}
</script>

<svelte:window
  onmousemove={(e) => { handleResizeMove(e); handleEventMouseMove(e); }}
  onmouseup={() => { handleResizeUp(); handleEventMouseUp(); }}
/>

<div
  class="absolute"
  data-calendar-event
  style="
    top: {top + yOffset}px;
    left: {leftPx}px;
    height: {displayHeight}px;
    width: {overlapWidth}px;
  "
  role="presentation"
  onclick={(e) => e.stopPropagation()}
  onmousedown={handleEventMouseDown}
>
  <ContextMenu.Root>
    <ContextMenu.Trigger class="block h-full w-full">
      <Draggable
        bind:this={draggableRef}
        class="relative h-full w-full"
        bounds={{
          top: -top,
          bottom: rowHeight * 24 - top - baseHeight,
          left: -colLeftPx,
          right: calendarWidth - colLeftPx - overlapWidth,
        }}
        grid={[colWidth, rowHeight / 4]}
        disabled={!event.draggable || resizing}
        onDragEnd={onDragEnd ?? null}
        onclick={null}
      >
        <div
          class="h-full rounded-sm border-l-2 px-1.5 pt-0.5 pb-1 text-left text-[11px] backdrop-blur-sm select-none overflow-hidden"
          style="background-color: color-mix(in srgb, {event.color} 15%, transparent); border-color: {event.color}; color: {event.color}"
        >
          {#if durationMin >= 30}
            <div class="truncate opacity-70">{formatTime(start)}</div>
          {/if}
          <div class="truncate font-semibold">{event.title}</div>
          {#if event.note && durationMin >= 30}
            <div class="truncate opacity-70">{event.note}</div>
          {/if}

          <!-- Resize handle -->
          {#if event.draggable && onResize}
            <div
              class="absolute bottom-0 left-0 right-0 h-2 cursor-ns-resize"
              role="presentation"
              onmousedown={startResize}
            ></div>
          {/if}
        </div>
      </Draggable>
    </ContextMenu.Trigger>

    <ContextMenu.Portal>
      <ContextMenu.Content
        class="z-50 min-w-[140px] rounded-lg border border-border bg-bg-secondary p-1 shadow-elevated"
      >
        {#if onViewTask}
          <ContextMenu.Item
            class="flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none hover:bg-overlay-light hover:text-text-primary"
            onSelect={() => onViewTask(event.taskId)}
          >
            <Icon src={FiExternalLink} size="12" className="text-text-muted" />
            Edit task
          </ContextMenu.Item>
        {/if}

        {#if event.blockId && onDelete}
          <ContextMenu.Separator class="my-1 border-t border-border" />
          <ContextMenu.Item
            class="flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 text-[12px] text-error outline-none hover:bg-error/10"
            onSelect={onDelete}
          >
            <Icon src={FiTrash2} size="12" />
            Delete block
          </ContextMenu.Item>
        {/if}
      </ContextMenu.Content>
    </ContextMenu.Portal>
  </ContextMenu.Root>
</div>

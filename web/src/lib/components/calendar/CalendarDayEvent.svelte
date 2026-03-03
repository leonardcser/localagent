<script lang="ts">
import type { CalendarEvent } from "$lib/calendar";
import { ContextMenu } from "bits-ui";
import { Icon } from "svelte-icons-pack";
import { FiExternalLink } from "svelte-icons-pack/fi";

interface Props {
  event: CalendarEvent;
  colWidth: number;
  onMove?: (colDelta: number) => void;
  onViewTask?: (taskId: string) => void;
}

let { event, colWidth, onMove, onViewTask }: Props = $props();

const DRAG_THRESHOLD = 4;

let dragging = $state(false);
let pending = $state(false);
let deltaX = $state(0);
let startX = 0;
let totalMovement = 0;

function handleMouseDown(e: MouseEvent) {
  if (e.button !== 0) return;
  e.preventDefault();
  pending = true;
  startX = e.clientX;
  deltaX = 0;
  totalMovement = 0;
}

function handleMove(e: MouseEvent) {
  if (!pending && !dragging) return;

  if (pending) {
    totalMovement += Math.abs(e.movementX) + Math.abs(e.movementY);
    if (totalMovement >= DRAG_THRESHOLD && onMove) {
      pending = false;
      dragging = true;
    } else if (totalMovement >= DRAG_THRESHOLD) {
      pending = false;
    }
    return;
  }

  deltaX = e.clientX - startX;
}

function handleUp() {
  if (pending) {
    pending = false;
    onViewTask?.(event.taskId);
    return;
  }
  if (!dragging) return;
  dragging = false;
  const colDelta = Math.round(deltaX / colWidth);
  if (colDelta !== 0) onMove?.(colDelta);
  deltaX = 0;
}
</script>

<svelte:window onmousemove={handleMove} onmouseup={handleUp} />

<ContextMenu.Root>
	<ContextMenu.Trigger class="block mb-0.5">
		<div
			class="truncate rounded-sm border-l-2 px-1.5 py-0.5 text-[11px] font-semibold select-none transition-opacity"
			class:cursor-grabbing={dragging}
			class:cursor-pointer={!dragging}
			class:opacity-60={dragging}
			style="
				background-color: color-mix(in srgb, {event.color} 15%, transparent);
				border-color: {event.color};
				color: {event.color};
				transform: translateX({deltaX}px);
				position: relative;
				z-index: {dragging ? 20 : 'auto'};
			"
			role="button"
			tabindex="0"
			onmousedown={handleMouseDown}
			onkeydown={() => {}}
		>
			{event.title}
		</div>
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
		</ContextMenu.Content>
	</ContextMenu.Portal>
</ContextMenu.Root>

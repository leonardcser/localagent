<script lang="ts">
import { addDays, isSameDay, weekdayShort } from "$lib/calendar";
import type { CalendarEvent } from "$lib/calendar";

interface Props {
  events: CalendarEvent[];
  viewStart: Date;
  currentDate: Date;
  onViewTask?: (taskId: string) => void;
}

let { events, viewStart, currentDate, onViewTask }: Props = $props();

let today = $state(new Date());

let currentMonth = $derived(currentDate.getMonth());

let numWeeks = $derived.by(() => {
  const lastDay = new Date(
    currentDate.getFullYear(),
    currentDate.getMonth() + 1,
    0,
  );
  // Need enough weeks to cover the last day of the month
  if (lastDay >= addDays(viewStart, 5 * 7)) return 6;
  return 5;
});

let totalDays = $derived(numWeeks * 7);

let eventsByDay = $derived.by(() => {
  const map = new Map<string, CalendarEvent[]>();
  for (const evt of events) {
    const d = new Date(evt.startMs);
    const key = `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
    const list = map.get(key) ?? [];
    list.push(evt);
    map.set(key, list);
  }
  return map;
});

function dayKey(d: Date): string {
  return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
}

const MAX_CHIPS = 3;
</script>

<div class="flex flex-1 flex-col overflow-hidden">
	<!-- Weekday header row -->
	<div class="grid shrink-0 border-b border-border bg-bg-secondary" style="grid-template-columns: repeat(7, 1fr)">
		{#each Array.from({ length: 7 }) as _, i}
			<div class="py-1.5 text-center text-[11px] font-medium text-text-muted">
				{weekdayShort(i)}
			</div>
		{/each}
	</div>

	<!-- Day grid -->
	<div class="grid flex-1 overflow-y-auto" style="grid-template-columns: repeat(7, 1fr); grid-template-rows: repeat({numWeeks}, 1fr)">
		{#each Array.from({ length: totalDays }) as _, dayIdx}
			{@const day = addDays(viewStart, dayIdx)}
			{@const isCurrentMonth = day.getMonth() === currentMonth}
			{@const isToday = isSameDay(day, today)}
			{@const dayEvts = eventsByDay.get(dayKey(day)) ?? []}
			{@const overflow = dayEvts.length - MAX_CHIPS}
			<div
				class="flex flex-col border-b border-r border-border/50 px-1 py-0.5 {isCurrentMonth ? 'bg-bg' : 'bg-bg-secondary/50'}"
			>
				<!-- Day number -->
				<div class="mb-0.5 flex items-center">
					<span
						class="flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-medium {isToday
							? 'bg-accent text-white'
							: isCurrentMonth
								? 'text-text-primary'
								: 'text-text-muted'}"
					>
						{day.getDate()}
					</span>
				</div>

				<!-- Event chips -->
				{#each dayEvts.slice(0, MAX_CHIPS) as evt}
					<button
						type="button"
						class="mb-px flex w-full items-center truncate rounded-sm border-l-2 px-1 py-px text-left text-[10px] leading-tight backdrop-blur-sm hover:opacity-80"
						style="background-color: color-mix(in srgb, {evt.color} 15%, transparent); border-color: {evt.color}; color: {evt.color}"
						onclick={() => onViewTask?.(evt.taskId)}
					>
						<span class="truncate">{evt.title}</span>
					</button>
				{/each}

				{#if overflow > 0}
					<div class="px-1 text-[9px] font-medium text-text-muted">
						+{overflow} more
					</div>
				{/if}
			</div>
		{/each}
	</div>
</div>

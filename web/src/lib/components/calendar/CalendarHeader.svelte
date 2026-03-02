<script lang="ts">
import { Icon } from "svelte-icons-pack";
import { FiChevronLeft, FiChevronRight } from "svelte-icons-pack/fi";
import { getWeekStart, addDays, isSameDay, weekdayShort } from "$lib/calendar";
import { cn } from "$lib/utils";

interface Props {
  indexColWidth: number;
  setDate: (date: Date) => void;
  date: Date;
}

let { indexColWidth, setDate, date }: Props = $props();

let today = $state(new Date());
let weekStart = $derived(getWeekStart(date));
let isCurrentWeek = $derived(isSameDay(getWeekStart(today), weekStart));

const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

function getISOWeek(d: Date): number {
  const tmp = new Date(d.getTime());
  tmp.setHours(0, 0, 0, 0);
  tmp.setDate(tmp.getDate() + 3 - ((tmp.getDay() + 6) % 7));
  const week1 = new Date(tmp.getFullYear(), 0, 4);
  return (
    1 +
    Math.round(
      ((tmp.getTime() - week1.getTime()) / 86400000 -
        3 +
        ((week1.getDay() + 6) % 7)) /
        7,
    )
  );
}
</script>

<div class="flex h-12 items-center justify-between border-b border-border bg-bg-secondary px-4">
	<div class="flex items-center gap-1.5 text-lg">
		<h2 class="font-bold text-text-primary">{MONTHS[date.getMonth()]}</h2>
		<h2 class="text-text-muted">{date.getFullYear()}</h2>
	</div>
	<div class="flex items-center gap-0.5">
		<button
			onclick={() => setDate(addDays(date, -7))}
			class="flex h-7 w-7 items-center justify-center rounded-md text-text-secondary transition-colors hover:bg-overlay-light"
		>
			<Icon src={FiChevronLeft} size="16" />
		</button>
		<button
			onclick={() => setDate(new Date())}
			class={cn(
				"rounded-md px-2.5 py-1 text-[12px] font-medium transition-colors",
				isCurrentWeek
					? "bg-accent text-white"
					: "bg-overlay-light text-text-secondary hover:bg-overlay-medium",
			)}
		>
			Today
		</button>
		<button
			onclick={() => setDate(addDays(date, 7))}
			class="flex h-7 w-7 items-center justify-center rounded-md text-text-secondary transition-colors hover:bg-overlay-light"
		>
			<Icon src={FiChevronRight} size="16" />
		</button>
	</div>
</div>
<div class="flex border-b border-border bg-bg-secondary py-1">
	<div
		class="flex items-center justify-center text-[11px] font-medium text-accent"
		style="width: {indexColWidth}px"
	>
		W{getISOWeek(weekStart)}
	</div>
	<div class="grid flex-1 grid-cols-7">
		{#each Array.from({ length: 7 }) as _, i}
			{@const day = addDays(weekStart, i)}
			{@const isToday = isSameDay(day, today)}
			<div class="flex items-center justify-center gap-1 text-center text-[12px] font-semibold text-text-secondary">
				<span>{weekdayShort(i)}</span>
				<span
					class={cn(
						"flex h-5 w-5 items-center justify-center rounded-full text-[11px]",
						isToday && "bg-accent text-white",
					)}
				>
					{day.getDate()}
				</span>
			</div>
		{/each}
	</div>
</div>

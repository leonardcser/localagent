<script lang="ts">
import { Icon } from "svelte-icons-pack";
import { FiChevronLeft, FiChevronRight } from "svelte-icons-pack/fi";
import { addDays, isSameDay, weekdayShort, getWeekStart } from "$lib/calendar";
import type { CalendarView } from "$lib/calendar";
import { cn } from "$lib/utils";

interface Props {
	indexColWidth: number;
	date: Date;
	view: CalendarView;
	numCols: number;
	viewStart: Date;
	navigate: (dir: -1 | 1) => void;
	goToToday: () => void;
	setView: (v: CalendarView) => void;
}

let { indexColWidth, date, view, numCols, viewStart, navigate, goToToday, setView }: Props =
	$props();

let today = $state(new Date());

let isCurrentPeriod = $derived.by(() => {
	const t = new Date(today.getFullYear(), today.getMonth(), today.getDate());
	const end = addDays(viewStart, numCols);
	return t >= viewStart && t < end;
});

const MONTHS = [
	"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
];

const MONTHS_SHORT = [
	"Jan", "Feb", "Mar", "Apr", "May", "Jun",
	"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
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

function headerTitle(): string {
	if (view === "day") {
		const d = viewStart;
		const day = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"][d.getDay()];
		return `${day}, ${MONTHS_SHORT[d.getMonth()]} ${d.getDate()}`;
	}
	if (view === "3day") {
		const end = addDays(viewStart, 2);
		if (viewStart.getMonth() === end.getMonth()) {
			return `${MONTHS_SHORT[viewStart.getMonth()]} ${viewStart.getDate()}–${end.getDate()}`;
		}
		return `${MONTHS_SHORT[viewStart.getMonth()]} ${viewStart.getDate()} – ${MONTHS_SHORT[end.getMonth()]} ${end.getDate()}`;
	}
	// week
	return `${MONTHS[viewStart.getMonth()]} ${viewStart.getFullYear()}`;
}

const views: { key: CalendarView; label: string }[] = [
	{ key: "day", label: "Day" },
	{ key: "3day", label: "3D" },
	{ key: "week", label: "Week" },
];
</script>

<!-- Top bar -->
<div class="flex h-11 items-center justify-between border-b border-border bg-bg-secondary px-3 gap-2">
	<div class="flex items-center gap-0.5">
		<button
			onclick={() => navigate(-1)}
			class="flex h-7 w-7 items-center justify-center rounded-md text-text-secondary transition-colors hover:bg-overlay-light"
		>
			<Icon src={FiChevronLeft} size="16" />
		</button>
		<button
			onclick={() => navigate(1)}
			class="flex h-7 w-7 items-center justify-center rounded-md text-text-secondary transition-colors hover:bg-overlay-light"
		>
			<Icon src={FiChevronRight} size="16" />
		</button>
		<button
			onclick={goToToday}
			class={cn(
				"ml-1 rounded-md px-2.5 py-1 text-[12px] font-medium transition-colors",
				isCurrentPeriod
					? "bg-accent text-white"
					: "bg-overlay-light text-text-secondary hover:bg-overlay-medium",
			)}
		>
			Today
		</button>
	</div>

	<span class="flex-1 text-center text-[13px] font-semibold text-text-primary truncate">
		{headerTitle()}
	</span>

	<!-- View switcher -->
	<div class="flex items-center rounded-lg border border-border bg-bg-tertiary p-0.5">
		{#each views as v}
			<button
				onclick={() => setView(v.key)}
				class={cn(
					"rounded-md px-2 py-0.5 text-[11px] font-medium transition-colors",
					view === v.key
						? "bg-accent text-white"
						: "text-text-muted hover:text-text-secondary",
				)}
			>
				{v.label}
			</button>
		{/each}
	</div>
</div>

<!-- Day headers -->
<div class="flex border-b border-border bg-bg-secondary py-1">
	<div
		class="flex items-center justify-center text-[11px] font-medium text-accent shrink-0"
		style="width: {indexColWidth}px"
	>
		{#if view === "week"}
			W{getISOWeek(viewStart)}
		{:else}
			&nbsp;
		{/if}
	</div>
	<div
		class="grid flex-1"
		style="grid-template-columns: repeat({numCols}, 1fr)"
	>
		{#each Array.from({ length: numCols }) as _, i}
			{@const day = addDays(viewStart, i)}
			{@const isToday = isSameDay(day, today)}
			<div
				class="flex items-center justify-center gap-1 text-center text-[12px] font-semibold text-text-secondary"
			>
				<span>{weekdayShort((day.getDay() + 6) % 7)}</span>
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

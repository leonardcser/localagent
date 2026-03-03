<script lang="ts">
import { Popover, Calendar } from "bits-ui";
import { parseDate, type DateValue } from "@internationalized/date";
import { Icon } from "svelte-icons-pack";
import {
  FiCalendar,
  FiChevronLeft,
  FiChevronRight,
  FiClock,
  FiPlus,
} from "svelte-icons-pack/fi";

let {
  oncreate,
}: {
  oncreate: (startMs: number, endMs: number) => void;
} = $props();

let open = $state(false);

// Use today as default date
function todayStr() {
  return new Date().toISOString().slice(0, 10);
}

function nowTimeStr() {
  const d = new Date();
  const h = d.getHours().toString().padStart(2, "0");
  const m = ((Math.ceil(d.getMinutes() / 15) * 15) % 60)
    .toString()
    .padStart(2, "0");
  return `${h}:${m}`;
}

function addHour(t: string): string {
  const [h, m] = t.split(":").map(Number);
  const next = h + 1;
  return `${next.toString().padStart(2, "0")}:${m.toString().padStart(2, "0")}`;
}

let dateStr = $state(todayStr());
let startTime = $state(nowTimeStr());
let endTime = $state(addHour(nowTimeStr()));

let calendarValue = $derived<DateValue | undefined>(
  dateStr ? parseDate(dateStr) : undefined,
);

function handleDateSelect(v: DateValue | undefined) {
  if (v) dateStr = v.toString();
}

function handleSubmit() {
  if (!dateStr || !startTime || !endTime) return;
  const start = new Date(`${dateStr}T${startTime}`).getTime();
  const end = new Date(`${dateStr}T${endTime}`).getTime();
  if (isNaN(start) || isNaN(end) || end <= start) return;
  oncreate(start, end);
  open = false;
  // Reset to fresh defaults for next use
  const now = nowTimeStr();
  startTime = now;
  endTime = addHour(now);
}

function handleOpen(o: boolean) {
  if (o) {
    dateStr = todayStr();
    const now = nowTimeStr();
    startTime = now;
    endTime = addHour(now);
  }
}

function formatDisplayDate(d: string): string {
  if (!d) return "";
  const today = new Date().toISOString().slice(0, 10);
  const tomorrow = new Date(Date.now() + 86400000).toISOString().slice(0, 10);
  if (d === today) return "Today";
  if (d === tomorrow) return "Tomorrow";
  return new Date(d + "T00:00:00").toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
  });
}
</script>

<Popover.Root bind:open onOpenChange={handleOpen}>
	<Popover.Trigger
		class="flex items-center gap-1 rounded-lg border border-dashed border-border px-2 py-1 text-[11px] text-text-muted outline-none hover:border-border-light hover:text-text-secondary"
	>
		<Icon src={FiPlus} size="11" />
		Add time block
	</Popover.Trigger>

	<Popover.Portal>
		<Popover.Content
			side="bottom"
			align="start"
			sideOffset={4}
			class="z-50 w-64 rounded-lg border border-border bg-bg-secondary p-3 shadow-elevated"
		>
			<!-- Calendar -->
			<Calendar.Root
				type="single"
				value={calendarValue}
				onValueChange={handleDateSelect}
				weekdayFormat="short"
				fixedWeeks
				class="w-full"
			>
				{#snippet children({ months, weekdays })}
					<Calendar.Header class="flex items-center justify-between px-1 pb-2">
						<Calendar.PrevButton
							class="flex h-6 w-6 items-center justify-center rounded-md text-text-muted hover:bg-overlay-light hover:text-text-secondary"
						>
							<Icon src={FiChevronLeft} size="14" />
						</Calendar.PrevButton>
						<Calendar.Heading class="text-[12px] font-medium text-text-primary" />
						<Calendar.NextButton
							class="flex h-6 w-6 items-center justify-center rounded-md text-text-muted hover:bg-overlay-light hover:text-text-secondary"
						>
							<Icon src={FiChevronRight} size="14" />
						</Calendar.NextButton>
					</Calendar.Header>

					{#each months as month}
						<Calendar.Grid class="w-full border-collapse">
							<Calendar.GridHead>
								<Calendar.GridRow class="flex w-full">
									{#each weekdays as day}
										<Calendar.HeadCell class="flex-1 pb-1 text-center text-[10px] text-text-muted">
											{day.slice(0, 2)}
										</Calendar.HeadCell>
									{/each}
								</Calendar.GridRow>
							</Calendar.GridHead>
							<Calendar.GridBody>
								{#each month.weeks as week}
									<Calendar.GridRow class="flex w-full">
										{#each week as date}
											<Calendar.Cell {date} month={month.value} class="flex-1 p-0 text-center">
												<Calendar.Day
													class="inline-flex h-7 w-7 items-center justify-center rounded-md text-[11px] text-text-secondary outline-none hover:bg-overlay-light data-[selected]:bg-accent data-[selected]:text-white data-[today]:font-semibold data-[today]:text-accent data-[selected]:data-[today]:text-white data-[outside-month]:text-text-muted/30"
												>
													{date.day}
												</Calendar.Day>
											</Calendar.Cell>
										{/each}
									</Calendar.GridRow>
								{/each}
							</Calendar.GridBody>
						</Calendar.Grid>
					{/each}
				{/snippet}
			</Calendar.Root>

			<!-- Time range -->
			<div class="mt-2 flex items-center gap-2 border-t border-border pt-2">
				<Icon src={FiClock} size="12" className="text-text-muted shrink-0" />
				<input
					type="time"
					bind:value={startTime}
					class="w-0 flex-1 rounded-md border border-border bg-bg-tertiary px-2 py-1 text-[11px] text-text-primary outline-none focus:border-accent"
				/>
				<span class="text-[11px] text-text-muted">–</span>
				<input
					type="time"
					bind:value={endTime}
					class="w-0 flex-1 rounded-md border border-border bg-bg-tertiary px-2 py-1 text-[11px] text-text-primary outline-none focus:border-accent"
				/>
			</div>

			<!-- Date summary + submit -->
			<div class="mt-2 flex items-center justify-between gap-2">
				<span class="flex items-center gap-1 text-[11px] text-text-muted">
					<Icon src={FiCalendar} size="11" />
					{formatDisplayDate(dateStr)}
				</span>
				<button
					onclick={handleSubmit}
					class="rounded-md bg-accent px-3 py-1 text-[11px] font-medium text-white hover:opacity-90"
				>
					Add
				</button>
			</div>
		</Popover.Content>
	</Popover.Portal>
</Popover.Root>

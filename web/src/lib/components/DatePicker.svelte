<script lang="ts">
import { Popover, Calendar } from "bits-ui";
import { parseDate, type DateValue } from "@internationalized/date";
import { Icon } from "svelte-icons-pack";
import {
  FiCalendar,
  FiChevronLeft,
  FiChevronRight,
  FiX,
  FiClock,
} from "svelte-icons-pack/fi";

let {
  value = "",
  onchange,
}: {
  value: string;
  onchange: (date: string) => void;
} = $props();

let open = $state(false);
let timeValue = $state("");

// Parse "YYYY-MM-DD" or "YYYY-MM-DDTHH:MM" into date part and time part
function parseDueDate(v: string): { date: string; time: string } {
  if (!v) return { date: "", time: "" };
  if (v.includes("T")) {
    const [d, t] = v.split("T");
    return { date: d, time: t };
  }
  return { date: v, time: "" };
}

let datePart = $derived(parseDueDate(value).date);

// Sync timeValue from prop on open
function syncTime() {
  timeValue = parseDueDate(value).time;
}

let calendarValue = $derived<DateValue | undefined>(
  datePart ? parseDate(datePart) : undefined,
);

function emitValue(date: string, time: string) {
  if (!date) {
    onchange("");
    return;
  }
  onchange(time ? `${date}T${time}` : date);
}

function handleDateSelect(v: DateValue | undefined) {
  if (v) {
    emitValue(v.toString(), timeValue);
  }
  open = false;
}

function handleTimeChange(e: Event) {
  const input = e.target as HTMLInputElement;
  timeValue = input.value;
  if (datePart) {
    emitValue(datePart, timeValue);
  }
}

function clearTime() {
  timeValue = "";
  if (datePart) {
    emitValue(datePart, "");
  }
}

function clear() {
  timeValue = "";
  onchange("");
}

function formatDisplay(v: string): string {
  const { date, time } = parseDueDate(v);
  if (!date) return "";
  const todayStr = new Date().toISOString().slice(0, 10);
  const tomorrowStr = new Date(Date.now() + 86400000)
    .toISOString()
    .slice(0, 10);
  let label: string;
  if (date === todayStr) label = "Today";
  else if (date === tomorrowStr) label = "Tomorrow";
  else {
    const d = new Date(date + "T00:00:00");
    label = d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
  }
  if (time) {
    const [h, m] = time.split(":");
    const hour = parseInt(h);
    const suffix = hour >= 12 ? "pm" : "am";
    const h12 = hour === 0 ? 12 : hour > 12 ? hour - 12 : hour;
    label += ` ${h12}:${m}${suffix}`;
  }
  return label;
}
</script>

<Popover.Root bind:open onOpenChange={(o) => { if (o) syncTime(); }}>
  <Popover.Trigger
    class="flex items-center gap-1.5 rounded-lg border border-border bg-bg-tertiary px-2 py-1 text-[12px] text-text-primary outline-none hover:border-border-light focus:border-accent"
  >
    <Icon src={FiCalendar} size="12" className="text-text-muted" />
    {#if value}
      <span>{formatDisplay(value)}</span>
      <button
        type="button"
        class="ml-0.5 rounded p-0.5 text-text-muted hover:text-text-secondary"
        onclick={(e) => {
          e.stopPropagation();
          clear();
        }}
      >
        <Icon src={FiX} size="10" />
      </button>
    {:else}
      <span class="text-text-muted">No date</span>
    {/if}
  </Popover.Trigger>

  <Popover.Portal>
    <Popover.Content
      side="bottom"
      align="end"
      sideOffset={4}
      class="z-50 rounded-lg border border-border bg-bg-secondary p-2 shadow-elevated"
    >
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
            <Calendar.Heading
              class="text-[12px] font-medium text-text-primary"
            />
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
                    <Calendar.HeadCell
                      class="flex-1 pb-1 text-center text-[10px] text-text-muted"
                    >
                      {day.slice(0, 2)}
                    </Calendar.HeadCell>
                  {/each}
                </Calendar.GridRow>
              </Calendar.GridHead>
              <Calendar.GridBody>
                {#each month.weeks as week}
                  <Calendar.GridRow class="flex w-full">
                    {#each week as date}
                      <Calendar.Cell
                        {date}
                        month={month.value}
                        class="flex-1 p-0 text-center"
                      >
                        <Calendar.Day
                          class="inline-flex h-7 w-7 items-center justify-center rounded-md text-[11px] text-text-secondary outline-none hover:bg-overlay-light data-[selected]:bg-accent data-[selected]:text-white data-[today]:font-semibold data-[today]:text-accent data-[selected]:data-[today]:text-white data-[outside-month]:text-text-muted/30 data-[disabled]:text-text-muted/30"
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

      <!-- Time input -->
      <div class="mt-2 flex items-center gap-2 border-t border-border pt-2">
        <Icon src={FiClock} size="12" className="text-text-muted" />
        <input
          type="time"
          value={timeValue}
          onchange={handleTimeChange}
          class="flex-1 rounded-md border border-border bg-bg-tertiary px-2 py-1 text-[11px] text-text-primary outline-none focus:border-accent"
        />
        {#if timeValue}
          <button
            type="button"
            class="rounded p-0.5 text-text-muted hover:text-text-secondary"
            onclick={clearTime}
          >
            <Icon src={FiX} size="10" />
          </button>
        {/if}
      </div>
    </Popover.Content>
  </Popover.Portal>
</Popover.Root>

<script lang="ts">
import { ContextMenu } from "bits-ui";
import { Calendar } from "bits-ui";
import { type DateValue, parseDate } from "@internationalized/date";
import type { Task } from "$lib/api";
import { taskStore } from "$lib/stores/task.svelte";
import { getBrowserLocale } from "$lib/utils";
import { Icon } from "svelte-icons-pack";
import {
  FiCheck,
  FiTrash2,
  FiCalendar,
  FiPlus,
  FiChevronLeft,
  FiChevronRight,
  FiSun,
  FiSunrise,
  FiChevronsRight,
  FiXCircle,
  FiX,
  FiBell,
  FiFlag,
} from "svelte-icons-pack/fi";

let {
  task,
  children,
  onAddSubtask,
}: {
  task: Task;
  children: import("svelte").Snippet;
  onAddSubtask?: (parentId: string) => void;
} = $props();

let calendarOpen = $state(false);

let calendarValue = $state<DateValue | undefined>(undefined);

function syncCalendarValue() {
  if (!task.due) {
    calendarValue = undefined;
    return;
  }
  const datePart = task.due.includes("T") ? task.due.split("T")[0] : task.due;
  calendarValue = parseDate(datePart);
}

async function setPriority(priority: string) {
  await taskStore.update(task.id, {
    priority: priority || undefined,
  } as Partial<Task>);
}

async function setDue(due: string | undefined) {
  await taskStore.update(task.id, { due } as Partial<Task>);
}

async function handleCalendarSelect(value: DateValue | undefined) {
  if (value) {
    await setDue(value.toString());
  }
  calendarOpen = false;
}

async function toggleReminder(key: string) {
  const current = task.reminders ?? [];
  const next = current.includes(key)
    ? current.filter((r) => r !== key)
    : [...current, key];
  await taskStore.update(task.id, { reminders: next } as Partial<Task>);
}

async function handleDelete() {
  await taskStore.remove(task.id);
}

function todayStr(): string {
  return new Date().toISOString().slice(0, 10);
}

function tomorrowStr(): string {
  return new Date(Date.now() + 86400000).toISOString().slice(0, 10);
}

function nextWeekStr(): string {
  return new Date(Date.now() + 7 * 86400000).toISOString().slice(0, 10);
}

function nextMonthStr(): string {
  const d = new Date();
  d.setMonth(d.getMonth() + 1);
  return d.toISOString().slice(0, 10);
}

function dueDate(): string | undefined {
  if (!task.due) return undefined;
  return task.due.includes("T") ? task.due.split("T")[0] : task.due;
}

const priorityFlags: {
  value: string;
  color: string;
  hoverColor: string;
  activeColor: string;
  label: string;
}[] = [
  {
    value: "high",
    color: "text-text-muted/40",
    hoverColor: "hover:text-error",
    activeColor: "text-error",
    label: "High",
  },
  {
    value: "medium",
    color: "text-text-muted/40",
    hoverColor: "hover:text-warning",
    activeColor: "text-warning",
    label: "Medium",
  },
  {
    value: "low",
    color: "text-text-muted/40",
    hoverColor: "hover:text-accent",
    activeColor: "text-accent",
    label: "Low",
  },
  {
    value: "",
    color: "text-text-muted/40",
    hoverColor: "hover:text-text-secondary",
    activeColor: "text-text-muted/40",
    label: "None",
  },
];

const dateOptions: {
  due: string | undefined;
  icon: typeof FiSun;
  color: string;
  label: string;
}[] = [
  { due: todayStr(), icon: FiSun, color: "text-warning", label: "Today" },
  {
    due: tomorrowStr(),
    icon: FiSunrise,
    color: "text-accent",
    label: "Tomorrow",
  },
  {
    due: nextWeekStr(),
    icon: FiChevronsRight,
    color: "text-success",
    label: "Next week",
  },
  {
    due: nextMonthStr(),
    icon: FiCalendar,
    color: "text-text-secondary",
    label: "Next month",
  },
];
</script>

<ContextMenu.Root
	onOpenChange={(open) => {
		if (open) syncCalendarValue();
		if (!open) calendarOpen = false;
	}}
>
	<ContextMenu.Trigger class="contents">
		{@render children()}
	</ContextMenu.Trigger>

	<ContextMenu.Portal>
		<ContextMenu.Content
			class="z-50 min-w-56 rounded-xl border border-border bg-bg-secondary p-1.5 shadow-elevated backdrop-blur-sm"
			sideOffset={5}
		>
			<!-- Date row -->
			<div class="px-2 pt-1 pb-0.5">
				<span class="text-[10px] font-medium uppercase tracking-wider text-text-muted/70">Date</span>
			</div>
			<div class="flex items-center gap-0.5 px-1.5 pb-1.5">
				{#each dateOptions as opt}
					{@const isActive = dueDate() === opt.due}
					<button
						class="flex h-8 w-8 items-center justify-center rounded-lg transition-colors
							{isActive ? 'bg-accent/15 ' + opt.color : 'text-text-muted hover:bg-overlay-light hover:text-text-secondary'}"
						title={opt.label}
						onclick={(e) => { e.preventDefault(); setDue(opt.due); }}
					>
						<Icon src={opt.icon} size="16" />
					</button>
				{/each}
				<!-- Calendar picker -->
				<ContextMenu.Sub bind:open={calendarOpen}>
					<ContextMenu.SubTrigger
						class="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted outline-none transition-colors hover:bg-overlay-light hover:text-text-secondary data-[state=open]:bg-overlay-light data-[state=open]:text-text-secondary"
					>
						<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
							<rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
							<line x1="16" y1="2" x2="16" y2="6" />
							<line x1="8" y1="2" x2="8" y2="6" />
							<line x1="3" y1="10" x2="21" y2="10" />
							<rect x="8" y="13" width="3" height="3" rx="0.5" fill="currentColor" stroke="none" />
						</svg>
					</ContextMenu.SubTrigger>
					<ContextMenu.SubContent
						class="z-50 rounded-xl border border-border bg-bg-secondary p-2 shadow-elevated backdrop-blur-sm"
						sideOffset={4}
					>
						<!-- svelte-ignore a11y_no_static_element_interactions -->
						<div role="presentation" onclick={(e) => e.stopPropagation()}>
							<Calendar.Root
								type="single"
								value={calendarValue}
								onValueChange={handleCalendarSelect}
								weekdayFormat="short"
								locale={getBrowserLocale()}
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
																	class="inline-flex h-6 w-6 items-center justify-center rounded-md text-[11px] text-text-secondary outline-none hover:bg-overlay-light data-[selected]:bg-accent data-[selected]:text-white data-[today]:font-semibold data-[today]:text-accent data-[selected]:data-[today]:text-white data-[outside-month]:text-text-muted/30 data-[disabled]:text-text-muted/30"
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
						</div>
					</ContextMenu.SubContent>
				</ContextMenu.Sub>
				<!-- Remove date -->
				{#if task.due}
					<button
						class="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-overlay-light hover:text-error"
						title="Remove date"
						onclick={(e) => { e.preventDefault(); setDue(undefined); }}
					>
						<Icon src={FiXCircle} size="15" />
					</button>
				{/if}
			</div>

			<!-- Priority row -->
			<div class="px-2 pb-0.5">
				<span class="text-[10px] font-medium uppercase tracking-wider text-text-muted/70">Priority</span>
			</div>
			<div class="flex items-center gap-1 px-1.5 pb-1">
				{#each priorityFlags as p}
					{@const isActive = (task.priority ?? "") === p.value}
					<button
						class="flex h-8 w-8 items-center justify-center rounded-lg transition-colors
							{isActive ? p.activeColor + ' bg-overlay-light' : p.color + ' ' + p.hoverColor + ' hover:bg-overlay-light'}"
						title={p.label}
						onclick={(e) => { e.preventDefault(); setPriority(p.value); }}
					>
						{#if p.value}
							<Icon src={FiFlag} size="16" className={isActive ? p.activeColor : ""} />
						{:else}
							<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class={isActive ? "text-text-secondary" : ""}>
								<path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z" />
								<line x1="4" y1="22" x2="4" y2="15" />
								<line x1="2" y1="2" x2="22" y2="22" opacity="0.5" />
							</svg>
						{/if}
					</button>
				{/each}
			</div>

			<ContextMenu.Separator class="mx-1.5 my-1 h-px bg-border" />

			<!-- Mark complete -->
			{#if task.status !== "done"}
				<ContextMenu.Item
					class="flex cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-2 text-[13px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-success"
					onSelect={() => taskStore.complete(task.id)}
				>
					<Icon src={FiCheck} size="15" />
					Mark complete
				</ContextMenu.Item>
			{/if}

			<!-- Add subtask -->
			{#if onAddSubtask && !task.parentId}
				<ContextMenu.Item
					class="flex cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-2 text-[13px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
					onSelect={() => onAddSubtask?.(task.id)}
				>
					<Icon src={FiPlus} size="15" />
					Add subtask
				</ContextMenu.Item>
			{/if}

			<!-- Reminders -->
			<ContextMenu.Sub>
				<ContextMenu.SubTrigger
					class="flex cursor-pointer items-center justify-between gap-2 rounded-lg px-2.5 py-2 text-[13px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary data-[state=open]:bg-overlay-light"
				>
					<span class="flex items-center gap-2.5">
						<Icon src={FiBell} size="15" />
						Reminders
						{#if (task.reminders ?? []).length > 0}
							<span class="rounded-full bg-accent/15 px-1.5 text-[10px] text-accent">{(task.reminders ?? []).length}</span>
						{/if}
					</span>
					<Icon src={FiChevronRight} size="13" className="text-text-muted" />
				</ContextMenu.SubTrigger>
				<ContextMenu.SubContent
					class="z-50 min-w-44 rounded-xl border border-border bg-bg-secondary p-1.5 shadow-elevated backdrop-blur-sm"
					sideOffset={4}
				>
					{#each [{ key: "0", label: "At due date" }, { key: "15m", label: "15 minutes before" }, { key: "30m", label: "30 minutes before" }, { key: "1h", label: "1 hour before" }, { key: "2h", label: "2 hours before" }, { key: "1d", label: "1 day before" }, { key: "2d", label: "2 days before" }, { key: "1w", label: "1 week before" }] as opt}
						{@const isTimeBased = !["0", "1d", "2d", "1w"].includes(opt.key)}
						{@const hasTime = task.due?.includes("T") ?? false}
						{#if !isTimeBased || hasTime}
							<ContextMenu.Item
								class="flex cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-2 text-[13px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
								onSelect={(e) => { e.preventDefault(); toggleReminder(opt.key); }}
							>
								<span
									class="flex h-3.5 w-3.5 items-center justify-center rounded border {(task.reminders ?? []).includes(opt.key) ? 'border-accent bg-accent' : 'border-border-light'}"
								>
									{#if (task.reminders ?? []).includes(opt.key)}
										<Icon src={FiCheck} size="8" className="text-white" />
									{/if}
								</span>
								{opt.label}
							</ContextMenu.Item>
						{/if}
					{/each}
				</ContextMenu.SubContent>
			</ContextMenu.Sub>

			<ContextMenu.Separator class="mx-1.5 my-1 h-px bg-border" />

			<!-- Delete -->
			<ContextMenu.Item
				class="flex cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-2 text-[13px] text-text-secondary outline-none data-[highlighted]:bg-danger/10 data-[highlighted]:text-danger"
				onSelect={handleDelete}
			>
				<Icon src={FiTrash2} size="15" />
				Delete
			</ContextMenu.Item>
		</ContextMenu.Content>
	</ContextMenu.Portal>
</ContextMenu.Root>

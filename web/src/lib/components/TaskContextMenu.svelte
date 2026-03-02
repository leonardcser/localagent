<script lang="ts">
import { ContextMenu } from "bits-ui";
import { Calendar } from "bits-ui";
import { type DateValue, parseDate } from "@internationalized/date";
import type { Task } from "$lib/api";
import { taskStore } from "$lib/stores/task.svelte";
import { Icon } from "svelte-icons-pack";
import {
	FiCheck,
	FiTrash2,
	FiCalendar,
	FiPlus,
	FiChevronLeft,
	FiChevronRight,
	FiSun,
	FiClock,
	FiX,
} from "svelte-icons-pack/fi";

let {
	task,
	children,
	onOpenDetail,
	onAddSubtask,
}: {
	task: Task;
	children: import("svelte").Snippet;
	onOpenDetail?: (task: Task) => void;
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

function isTaskOverdue(): boolean {
	if (!task.due || task.status === "done") return false;
	const datePart = task.due.includes("T") ? task.due.split("T")[0] : task.due;
	return datePart < new Date().toISOString().slice(0, 10);
}

async function setPriority(priority: string) {
	await taskStore.update(task.id, {
		priority: priority || undefined,
	} as Partial<Task>);
}

async function setStatus(status: string) {
	if (status === "done") {
		await taskStore.complete(task.id);
	} else {
		await taskStore.update(task.id, { status } as Partial<Task>);
	}
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

const priorities = [
	{ value: "", label: "None", color: "" },
	{ value: "low", label: "Low", color: "bg-text-muted" },
	{ value: "medium", label: "Medium", color: "bg-warning" },
	{ value: "high", label: "High", color: "bg-error" },
];

const statuses = [
	{ value: "todo", label: "To Do" },
	{ value: "doing", label: "In Progress" },
	{ value: "done", label: "Done" },
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
			class="z-50 min-w-52 rounded-lg border border-border bg-bg-secondary p-1 shadow-elevated"
			sideOffset={5}
		>
			<!-- Open detail -->
			{#if onOpenDetail}
				<ContextMenu.Item
					class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
					onSelect={() => onOpenDetail?.(task)}
				>
					<svg
						width="14"
						height="14"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					>
						<path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
						<polyline points="15 3 21 3 21 9" />
						<line x1="10" y1="14" x2="21" y2="3" />
					</svg>
					Open
				</ContextMenu.Item>
				<ContextMenu.Separator class="mx-1 my-1 h-px bg-border" />
			{/if}

			<!-- Reschedule to today (shown for overdue tasks) -->
			{#if isTaskOverdue()}
				<ContextMenu.Item
					class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-warning outline-none data-[highlighted]:bg-warning/10"
					onSelect={() => setDue(todayStr())}
				>
					<Icon src={FiSun} size="14" />
					Reschedule to today
				</ContextMenu.Item>
				<ContextMenu.Separator class="mx-1 my-1 h-px bg-border" />
			{/if}

			<!-- Status -->
			<ContextMenu.Sub>
				<ContextMenu.SubTrigger
					class="flex cursor-pointer items-center justify-between gap-2 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary data-[state=open]:bg-overlay-light"
				>
					<span class="flex items-center gap-2.5">
						<svg
							width="14"
							height="14"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
						>
							<circle cx="12" cy="12" r="10" />
							{#if task.status === "done"}
								<polyline points="9 12 11.5 14.5 16 10" />
							{:else if task.status === "doing"}
								<path d="M12 6v6l3 3" />
							{/if}
						</svg>
						Status
					</span>
					<Icon src={FiChevronRight} size="12" className="text-text-muted" />
				</ContextMenu.SubTrigger>
				<ContextMenu.SubContent
					class="z-50 min-w-36 rounded-lg border border-border bg-bg-secondary p-1 shadow-elevated"
					sideOffset={4}
				>
					<ContextMenu.RadioGroup
						value={task.status}
						onValueChange={(v) => setStatus(v)}
					>
						{#each statuses as s}
							<ContextMenu.RadioItem
								value={s.value}
								class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
							>
								{#snippet children({ checked })}
									<span
										class="flex h-3.5 w-3.5 items-center justify-center rounded-full border border-border-light {checked ? 'bg-accent border-accent' : ''}"
									>
										{#if checked}
											<Icon src={FiCheck} size="8" className="text-white" />
										{/if}
									</span>
									{s.label}
								{/snippet}
							</ContextMenu.RadioItem>
						{/each}
					</ContextMenu.RadioGroup>
				</ContextMenu.SubContent>
			</ContextMenu.Sub>

			<!-- Priority -->
			<ContextMenu.Sub>
				<ContextMenu.SubTrigger
					class="flex cursor-pointer items-center justify-between gap-2 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary data-[state=open]:bg-overlay-light"
				>
					<span class="flex items-center gap-2.5">
						<svg
							width="14"
							height="14"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
						>
							<path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z" />
							<line x1="4" y1="22" x2="4" y2="15" />
						</svg>
						Priority
					</span>
					<Icon src={FiChevronRight} size="12" className="text-text-muted" />
				</ContextMenu.SubTrigger>
				<ContextMenu.SubContent
					class="z-50 min-w-36 rounded-lg border border-border bg-bg-secondary p-1 shadow-elevated"
					sideOffset={4}
				>
					<ContextMenu.RadioGroup
						value={task.priority ?? ""}
						onValueChange={(v) => setPriority(v)}
					>
						{#each priorities as p}
							<ContextMenu.RadioItem
								value={p.value}
								class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
							>
								{#snippet children({ checked })}
									<span
										class="flex h-3.5 w-3.5 items-center justify-center rounded-full border border-border-light {checked ? 'bg-accent border-accent' : ''}"
									>
										{#if checked}
											<Icon src={FiCheck} size="8" className="text-white" />
										{/if}
									</span>
									<span class="flex items-center gap-2">
										{#if p.color}
											<span class="h-2 w-2 rounded-full {p.color}"></span>
										{/if}
										{p.label}
									</span>
								{/snippet}
							</ContextMenu.RadioItem>
						{/each}
					</ContextMenu.RadioGroup>
				</ContextMenu.SubContent>
			</ContextMenu.Sub>

			<!-- Schedule -->
			<ContextMenu.Sub bind:open={calendarOpen}>
				<ContextMenu.SubTrigger
					class="flex cursor-pointer items-center justify-between gap-2 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary data-[state=open]:bg-overlay-light"
				>
					<span class="flex items-center gap-2.5">
						<Icon src={FiCalendar} size="14" />
						Schedule
					</span>
					<Icon src={FiChevronRight} size="12" className="text-text-muted" />
				</ContextMenu.SubTrigger>
				<ContextMenu.SubContent
					class="z-50 min-w-52 rounded-lg border border-border bg-bg-secondary p-1 shadow-elevated"
					sideOffset={4}
				>
					<!-- Quick date options -->
					<ContextMenu.Item
						class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
						onSelect={() => setDue(todayStr())}
					>
						<Icon src={FiSun} size="13" className="text-warning" />
						Today
						{#if task.due === todayStr()}
							<Icon src={FiCheck} size="11" className="ml-auto text-accent" />
						{/if}
					</ContextMenu.Item>
					<ContextMenu.Item
						class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
						onSelect={() => setDue(tomorrowStr())}
					>
						<Icon src={FiClock} size="13" className="text-accent" />
						Tomorrow
						{#if task.due === tomorrowStr()}
							<Icon src={FiCheck} size="11" className="ml-auto text-accent" />
						{/if}
					</ContextMenu.Item>
					<ContextMenu.Item
						class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
						onSelect={() => setDue(nextWeekStr())}
					>
						<Icon src={FiCalendar} size="13" className="text-success" />
						Next Week
					</ContextMenu.Item>
					{#if task.due}
						<ContextMenu.Item
							class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
							onSelect={() => setDue(undefined)}
						>
							<Icon src={FiX} size="13" className="text-text-muted" />
							Remove date
						</ContextMenu.Item>
					{/if}

					<ContextMenu.Separator class="mx-1 my-1 h-px bg-border" />

					<!-- Calendar -->
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div class="p-1" role="presentation" onclick={(e) => e.stopPropagation()}>
						<Calendar.Root
							type="single"
							value={calendarValue}
							onValueChange={handleCalendarSelect}
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

			<ContextMenu.Separator class="mx-1 my-1 h-px bg-border" />

			<!-- Complete -->
			{#if task.status !== "done"}
				<ContextMenu.Item
					class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-success"
					onSelect={() => taskStore.complete(task.id)}
				>
					<Icon src={FiCheck} size="14" />
					Mark complete
				</ContextMenu.Item>
			{/if}

			<!-- Add subtask -->
			{#if onAddSubtask && !task.parentId}
				<ContextMenu.Item
					class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
					onSelect={() => onAddSubtask?.(task.id)}
				>
					<Icon src={FiPlus} size="14" />
					Add subtask
				</ContextMenu.Item>
			{/if}

			<ContextMenu.Separator class="mx-1 my-1 h-px bg-border" />

			<!-- Delete -->
			<ContextMenu.Item
				class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-danger/10 data-[highlighted]:text-danger"
				onSelect={handleDelete}
			>
				<Icon src={FiTrash2} size="14" />
				Delete
			</ContextMenu.Item>
		</ContextMenu.Content>
	</ContextMenu.Portal>
</ContextMenu.Root>

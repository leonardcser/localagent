<script lang="ts">
import { Select } from "bits-ui";
import { Icon } from "svelte-icons-pack";
import {
  FiX,
  FiTrash2,
  FiArrowLeft,
  FiPlus,
  FiCheck,
  FiList,
  FiCalendar,
  FiTag,
  FiCornerDownRight,
  FiChevronDown,
  FiRepeat,
  FiClock,
  FiBell,
} from "svelte-icons-pack/fi";
import { taskStore } from "$lib/stores/task.svelte";
import { blockStore } from "$lib/stores/block.svelte";
import DatePicker from "$lib/components/DatePicker.svelte";
import RecurrencePicker from "$lib/components/RecurrencePicker.svelte";
import BlockPicker from "$lib/components/BlockPicker.svelte";
import type { Task } from "$lib/api";
import { formatTime24 } from "$lib/utils";

interface Props {
  task: Task | null; // null = add mode
  parentId?: string;
  onClose: () => void;
  onSelectTask?: (task: Task) => void; // navigate to subtask
  onAddSubtask?: (parentId: string) => void;
}

let {
  task,
  parentId = "",
  onClose,
  onSelectTask,
  onAddSubtask,
}: Props = $props();

let mode = $derived<"add" | "edit">(task === null ? "add" : "edit");

// Form state — synced from task prop via $effect below
let title = $state("");
let description = $state("");
let priority = $state("");
let due = $state("");
let recurrence = $state("");
let tagsRaw = $state("");
let reminders = $state<string[]>([]);
let status = $state("todo");
let selectedParentId = $state("");

// Re-sync when task changes (e.g. navigating between tasks)
$effect(() => {
  flushPendingSave();
  title = task?.title ?? "";
  description = task?.description ?? "";
  priority = task?.priority ?? "";
  due = task?.due ?? "";
  recurrence = task?.recurrence ?? "";
  tagsRaw = (task?.tags ?? []).join(", ");
  reminders = task?.reminders ?? [];
  status = task?.status ?? "todo";
  selectedParentId = task?.parentId ?? parentId;
});

const statusOptions = [
  { value: "todo", label: "To Do" },
  { value: "doing", label: "In Progress" },
  { value: "done", label: "Done" },
];

const priorityOptions = [
  { value: "", label: "None" },
  { value: "low", label: "Low" },
  { value: "medium", label: "Medium" },
  { value: "high", label: "High" },
];

function parseTags(raw: string): string[] | undefined {
  const tags = raw
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean);
  return tags.length > 0 ? tags : undefined;
}

function priorityColor(p?: string): string {
  if (p === "high") return "bg-error";
  if (p === "medium") return "bg-warning";
  if (p === "low") return "bg-text-muted";
  return "";
}

// Auto-save
async function autoSave(patch: Partial<Task>) {
  if (mode !== "edit" || !task) return;
  await taskStore.update(task.id, patch);
}

let saveTimer: ReturnType<typeof setTimeout> | null = null;
let pendingPatch: Partial<Task> | null = null;
function debouncedAutoSave(patch: Partial<Task>) {
  if (saveTimer) clearTimeout(saveTimer);
  pendingPatch = patch;
  saveTimer = setTimeout(() => {
    pendingPatch = null;
    autoSave(patch);
  }, 400);
}
function flushPendingSave() {
  if (saveTimer) {
    clearTimeout(saveTimer);
    saveTimer = null;
  }
  if (pendingPatch) {
    const patch = pendingPatch;
    pendingPatch = null;
    autoSave(patch);
  }
}

// Add mode submit
async function handleAddSubmit(e: SubmitEvent) {
  e.preventDefault();
  if (!title.trim()) return;
  await taskStore.add({
    title: title.trim(),
    description: description.trim() || undefined,
    priority: priority || undefined,
    due: due || undefined,
    recurrence: recurrence || undefined,
    tags: parseTags(tagsRaw),
    status,
    parentId: selectedParentId || undefined,
  });
  onClose();
}

async function handleDelete() {
  if (mode === "edit" && task) {
    await taskStore.remove(task.id);
    onClose();
  }
}

let taskBlocks = $derived(task ? blockStore.forTask(task.id) : []);
let subtasks = $derived(task ? taskStore.subtasksOf(task.id) : []);
</script>

{#snippet selectIndicator(selected: boolean)}
	<span
		class="flex h-3.5 w-3.5 items-center justify-center rounded-full border border-border-light {selected ? 'bg-accent border-accent' : ''}"
	>
		{#if selected}
			<Icon src={FiCheck} size="8" className="text-white" />
		{/if}
	</span>
{/snippet}

{#snippet body(mobile: boolean)}
	{@const sz = mobile ? "text-[14px]" : "text-[12px]"}
	{@const szLabel = mobile ? "text-[14px]" : "text-[12px]"}
	{@const gap = mobile ? "gap-3" : "gap-2"}
	{@const iconSz = mobile ? "16" : "13"}
	{@const py = mobile ? "py-3" : "py-2.5"}

	<div class="flex flex-1 flex-col gap-4 overflow-y-auto p-4">
		<input
			type="text"
			bind:value={title}
			placeholder="Task title"
			class="bg-transparent {mobile ? 'text-[17px] font-semibold' : 'text-[15px] font-medium'} text-text-primary placeholder:text-text-muted outline-none"
			onblur={() => {
				if (title.trim()) debouncedAutoSave({ title: title.trim() });
			}}
		/>

		<textarea
			bind:value={description}
			placeholder="Add notes..."
			rows={mobile ? 4 : 3}
			class="resize-none bg-transparent {mobile ? 'text-[14px]' : 'text-[13px]'} text-text-secondary placeholder:text-text-muted outline-none"
			onblur={() => debouncedAutoSave({ description: description.trim() || undefined })}
		></textarea>

		<div class="flex flex-col gap-0 border-t border-border pt-2">
			<!-- Status -->
			<div class="flex items-center justify-between {py} border-b border-border/50">
				<span class="flex items-center {gap} {szLabel} text-text-secondary">
					<Icon src={FiList} size={iconSz} className="text-text-muted" />
					Status
				</span>
				<Select.Root
					type="single"
					value={status}
					onValueChange={(v) => {
						status = v;
						if (v === "done" && task) {
							taskStore.complete(task.id);
						} else {
							autoSave({ status: v });
						}
					}}
				>
					<Select.Trigger
						class="flex items-center gap-1.5 rounded-lg border border-border bg-bg-tertiary px-2 py-1 {sz} text-text-primary outline-none hover:border-border-light focus:border-accent"
					>
						{statusOptions.find((s) => s.value === status)?.label ?? "To Do"}
						<Icon src={FiChevronDown} size="11" className="text-text-muted" />
					</Select.Trigger>
					<Select.Portal>
						<Select.Content
							class="z-50 min-w-32 rounded-lg border border-border bg-bg-secondary p-1 shadow-elevated"
							sideOffset={4}
						>
							<Select.Viewport>
								{#each statusOptions as opt}
									<Select.Item
										value={opt.value}
										label={opt.label}
										class="flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 {sz} text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
									>
										{#snippet children({ selected })}
											{@render selectIndicator(selected)}
											{opt.label}
										{/snippet}
									</Select.Item>
								{/each}
							</Select.Viewport>
						</Select.Content>
					</Select.Portal>
				</Select.Root>
			</div>

			<!-- Priority -->
			<div class="flex items-center justify-between {py} border-b border-border/50">
				<span class="flex items-center {gap} {szLabel} text-text-secondary">
					<svg width={iconSz} height={iconSz} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-text-muted">
						<path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z" />
						<line x1="4" y1="22" x2="4" y2="15" />
					</svg>
					Priority
				</span>
				<Select.Root
					type="single"
					value={priority}
					onValueChange={(v) => {
						priority = v;
						autoSave({ priority: v || undefined } as Partial<Task>);
					}}
				>
					<Select.Trigger
						class="flex items-center gap-1.5 rounded-lg border border-border bg-bg-tertiary px-2 py-1 {sz} text-text-primary outline-none hover:border-border-light focus:border-accent"
					>
						<span class="flex items-center gap-1.5">
							{#if priority}
								<span class="h-2 w-2 rounded-full {priorityColor(priority)}"></span>
							{/if}
							{priorityOptions.find((p) => p.value === priority)?.label ?? "None"}
						</span>
						<Icon src={FiChevronDown} size="11" className="text-text-muted" />
					</Select.Trigger>
					<Select.Portal>
						<Select.Content
							class="z-50 min-w-32 rounded-lg border border-border bg-bg-secondary p-1 shadow-elevated"
							sideOffset={4}
						>
							<Select.Viewport>
								{#each priorityOptions as opt}
									<Select.Item
										value={opt.value}
										label={opt.label}
										class="flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 {sz} text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
									>
										{#snippet children({ selected })}
											{@render selectIndicator(selected)}
											<span class="flex items-center gap-1.5">
												{#if opt.value}
													<span class="h-2 w-2 rounded-full {priorityColor(opt.value)}"></span>
												{/if}
												{opt.label}
											</span>
										{/snippet}
									</Select.Item>
								{/each}
							</Select.Viewport>
						</Select.Content>
					</Select.Portal>
				</Select.Root>
			</div>

			<!-- Due Date -->
			<div class="flex items-center justify-between {py} border-b border-border/50">
				<span class="flex items-center {gap} {szLabel} text-text-secondary">
					<Icon src={FiCalendar} size={iconSz} className="text-text-muted" />
					Due Date
				</span>
				<DatePicker
					value={due}
					onchange={(v) => {
						due = v;
						autoSave({ due: v || undefined } as Partial<Task>);
					}}
				/>
			</div>

			<!-- Reminders -->
			<div class="flex items-center justify-between {py} border-b border-border/50">
				<span class="flex items-center {gap} {szLabel} text-text-secondary">
					<Icon src={FiBell} size={iconSz} className="text-text-muted" />
					Reminders
				</span>
				<div class="flex flex-wrap justify-end gap-1">
					{#each [{ key: "0", label: "At due" }, { key: "15m", label: "15m" }, { key: "30m", label: "30m" }, { key: "1h", label: "1h" }, { key: "2h", label: "2h" }, { key: "1d", label: "1d" }, { key: "2d", label: "2d" }, { key: "1w", label: "1w" }] as opt}
						{@const active = reminders.includes(opt.key)}
						{@const isTimeBased = !["0", "1d", "2d", "1w"].includes(opt.key)}
						{@const hasTime = due.includes("T")}
						{#if !isTimeBased || hasTime}
							<button
								type="button"
								class="rounded-md border px-1.5 py-0.5 text-[10px] font-medium transition-colors
									{active
										? 'border-accent bg-accent/15 text-accent'
										: 'border-border bg-bg-tertiary text-text-muted hover:border-border-light hover:text-text-secondary'}"
								onclick={() => {
									if (active) {
										reminders = reminders.filter((r) => r !== opt.key);
									} else {
										reminders = [...reminders, opt.key];
									}
									autoSave({ reminders } as Partial<Task>);
								}}
							>
								{opt.label}
							</button>
						{/if}
					{/each}
				</div>
			</div>

			<!-- Recurrence -->
			<div class="flex items-center justify-between {py} border-b border-border/50">
				<span class="flex items-center {gap} {szLabel} text-text-secondary">
					<Icon src={FiRepeat} size={iconSz} className="text-text-muted" />
					Repeat
				</span>
				<RecurrencePicker
					value={recurrence}
					onchange={(v) => {
						recurrence = v;
						autoSave({ recurrence: v || undefined } as Partial<Task>);
					}}
				/>
			</div>

			<!-- Tags -->
			<div class="flex items-center justify-between {py} border-b border-border/50">
				<span class="flex items-center {gap} {szLabel} text-text-secondary">
					<Icon src={FiTag} size={iconSz} className="text-text-muted" />
					Tags
				</span>
				<input
					type="text"
					bind:value={tagsRaw}
					placeholder="work, personal"
					class="{mobile ? 'w-40' : 'w-36'} rounded-lg border border-border bg-bg-tertiary px-2 py-1 {sz} text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
					onblur={() => {
						const tags = parseTags(tagsRaw);
						autoSave({ tags: tags ?? [] } as Partial<Task>);
					}}
				/>
			</div>

			<!-- Parent -->
			<div class="flex items-center justify-between {py}">
				<span class="flex items-center {gap} {szLabel} text-text-secondary">
					<Icon src={FiCornerDownRight} size={iconSz} className="text-text-muted" />
					Parent
				</span>
				<Select.Root
					type="single"
					value={selectedParentId}
					onValueChange={(v) => {
						selectedParentId = v;
						autoSave({ parentId: v || undefined } as Partial<Task>);
					}}
				>
					<Select.Trigger
						class="{mobile ? 'w-40' : 'w-36'} flex items-center gap-1.5 truncate rounded-lg border border-border bg-bg-tertiary px-2 py-1 {sz} text-text-primary outline-none hover:border-border-light focus:border-accent"
					>
						<span class="flex-1 truncate text-left">
							{selectedParentId
								? (taskStore.tasks.find((t) => t.id === selectedParentId)?.title ?? "None")
								: "None"}
						</span>
						<Icon src={FiChevronDown} size="11" className="shrink-0 text-text-muted" />
					</Select.Trigger>
					<Select.Portal>
						<Select.Content
							class="z-50 max-h-48 min-w-40 rounded-lg border border-border bg-bg-secondary p-1 shadow-elevated"
							sideOffset={4}
						>
							<Select.Viewport class="max-h-44 overflow-y-auto">
								<Select.Item
									value=""
									label="None"
									class="flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 {sz} text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
								>
									{#snippet children({ selected })}
										{@render selectIndicator(selected)}
										None
									{/snippet}
								</Select.Item>
								{#each taskStore.tasks.filter((t) => t.id !== task?.id && !t.parentId) as t}
									<Select.Item
										value={t.id}
										label={t.title}
										class="flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 {sz} text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
									>
										{#snippet children({ selected })}
											{@render selectIndicator(selected)}
											<span class="truncate">{t.title}</span>
										{/snippet}
									</Select.Item>
								{/each}
							</Select.Viewport>
						</Select.Content>
					</Select.Portal>
				</Select.Root>
			</div>
		</div>

		<!-- Time Blocks (edit mode only) -->
		{#if mode === "edit" && task}
			<div class="flex flex-col gap-1 border-t border-border pt-3">
				<div class="flex items-center justify-between">
					<span class="text-[11px] font-semibold uppercase tracking-widest text-text-muted">Time Blocks</span>
					<BlockPicker oncreate={(startMs, endMs) => blockStore.add({ taskId: task!.id, startAtMs: startMs, endAtMs: endMs })} />
				</div>
				{#each taskBlocks as block}
					<div class="flex items-center justify-between rounded-lg px-2 {mobile ? 'py-2' : 'py-1.5'} hover:bg-overlay-light">
						<span class="{mobile ? 'text-[13px]' : 'text-[11px]'} text-text-secondary">
							<Icon src={FiClock} size="11" className="inline-block text-text-muted mr-1" />
							{new Date(block.startAtMs).toLocaleDateString(undefined, { month: "short", day: "numeric" })}
							{formatTime24(new Date(block.startAtMs))}
							– {formatTime24(new Date(block.endAtMs))}
						</span>
						<button
							onclick={() => blockStore.remove(block.id)}
							class="text-text-muted hover:text-error transition-colors"
						>
							<Icon src={FiX} size="12" />
						</button>
					</div>
				{/each}
			</div>
		{/if}

		<!-- Subtasks (edit mode only) -->
		{#if mode === "edit" && task}
			{#if subtasks.length > 0}
				<div class="flex flex-col gap-1 border-t border-border pt-3">
					<span class="text-[11px] font-semibold uppercase tracking-widest text-text-muted">Subtasks</span>
					{#each subtasks as sub}
						<button
							onclick={() => onSelectTask?.(sub)}
							class="flex items-center {gap} rounded-lg px-2 {mobile ? 'py-2' : 'py-1.5'} text-left transition-colors hover:bg-overlay-light"
						>
							<span
								role="checkbox"
								tabindex="-1"
								aria-checked={sub.status === "done"}
								onclick={(e) => { e.stopPropagation(); sub.status === "done" ? taskStore.update(sub.id, { status: "todo" }) : taskStore.complete(sub.id); }}
								onkeydown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); e.stopPropagation(); sub.status === "done" ? taskStore.update(sub.id, { status: "todo" }) : taskStore.complete(sub.id); } }}
								class="flex {mobile ? 'h-5 w-5' : 'h-4 w-4'} shrink-0 items-center justify-center rounded-full border-[1.5px] transition-colors
									{sub.status === 'done' ? 'border-success bg-success text-white' : 'border-border-light text-transparent hover:border-text-muted'}"
							>
								<Icon src={FiCheck} size={mobile ? "10" : "9"} />
							</span>
							<span class="truncate {mobile ? 'text-[14px]' : 'text-[12px]'} {sub.status === 'done' ? 'text-text-muted line-through' : 'text-text-primary'}">
								{sub.title}
							</span>
						</button>
					{/each}
				</div>
			{/if}
			<button
				type="button"
				onclick={() => onAddSubtask?.(task!.id)}
				class="flex items-center {mobile ? 'gap-2 text-[14px]' : 'gap-1.5 text-[12px]'} text-text-muted hover:text-accent transition-colors"
			>
				<Icon src={FiPlus} size={mobile ? "14" : "12"} />
				Add subtask
			</button>
		{/if}
	</div>
{/snippet}

<!-- Desktop panel (w-80 sidebar) -->
<div class="hidden w-80 shrink-0 flex-col border-l border-border bg-bg md:flex">
	{#if mode === "add"}
		<form onsubmit={handleAddSubmit} class="flex flex-1 flex-col overflow-hidden">
			<div class="flex items-center justify-between border-b border-border px-4 py-3">
				<h2 class="text-[14px] font-semibold text-text-primary">New Task</h2>
				<button
					type="button"
					onclick={onClose}
					class="flex h-7 w-7 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-text-secondary"
				>
					<Icon src={FiX} size="14" />
				</button>
			</div>
			{@render body(false)}
			<div class="border-t border-border px-4 py-3">
				<button
					type="submit"
					disabled={!title.trim()}
					class="w-full rounded-lg bg-accent py-2 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40"
				>
					Add Task
				</button>
			</div>
		</form>
	{:else}
		<div class="flex flex-1 flex-col overflow-hidden">
			<div class="flex items-center justify-between border-b border-border px-4 py-3">
				<h2 class="text-[14px] font-semibold text-text-primary">Edit Task</h2>
				<div class="flex items-center gap-1">
					<button
						type="button"
						onclick={handleDelete}
						class="flex h-7 w-7 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-danger"
						title="Delete task"
					>
						<Icon src={FiTrash2} size="14" />
					</button>
					<button
						type="button"
						onclick={onClose}
						class="flex h-7 w-7 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-text-secondary"
					>
						<Icon src={FiX} size="14" />
					</button>
				</div>
			</div>
			{@render body(false)}
		</div>
	{/if}
</div>

<!-- Mobile full-screen panel -->
<div class="fixed inset-0 z-40 flex flex-col bg-bg md:hidden">
	{#if mode === "add"}
		<form onsubmit={handleAddSubmit} class="flex flex-1 flex-col overflow-hidden">
			<div
				class="flex items-center gap-2 border-b border-border px-3 py-2"
				style="padding-top: max(env(safe-area-inset-top, 0px), 10px)"
			>
				<button
					type="button"
					onclick={onClose}
					class="flex h-9 w-9 items-center justify-center rounded-lg text-text-secondary hover:bg-overlay-light"
				>
					<Icon src={FiArrowLeft} size="18" />
				</button>
				<span class="flex-1"></span>
				<button
					type="submit"
					disabled={!title.trim()}
					class="flex h-9 items-center rounded-lg bg-accent px-4 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40"
				>
					Add
				</button>
			</div>
			{@render body(true)}
		</form>
	{:else}
		<div class="flex flex-1 flex-col overflow-hidden">
			<div
				class="flex items-center gap-2 border-b border-border px-3 py-2"
				style="padding-top: max(env(safe-area-inset-top, 0px), 10px)"
			>
				<button
					type="button"
					onclick={onClose}
					class="flex h-9 w-9 items-center justify-center rounded-lg text-text-secondary hover:bg-overlay-light"
				>
					<Icon src={FiArrowLeft} size="18" />
				</button>
				<span class="flex-1"></span>
				<button
					type="button"
					onclick={handleDelete}
					class="flex h-9 w-9 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-danger"
				>
					<Icon src={FiTrash2} size="17" />
				</button>
			</div>
			{@render body(true)}
		</div>
	{/if}
</div>

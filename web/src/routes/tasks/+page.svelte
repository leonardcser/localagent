<script lang="ts">
import { onMount } from "svelte";
import { taskStore } from "$lib/stores/task.svelte";
import type { Task } from "$lib/api";
import { Icon } from "svelte-icons-pack";
import {
	FiSearch,
	FiPlus,
	FiCheck,
	FiTrash2,
	FiList,
	FiColumns,
	FiX,
	FiChevronDown,
	FiChevronUp,
	FiCalendar,
	FiTag,
} from "svelte-icons-pack/fi";

// Modal state
let modalOpen = $state(false);
let modalMode = $state<"add" | "edit">("add");
let modalId = $state("");
let modalTitle = $state("");
let modalDescription = $state("");
let modalPriority = $state("");
let modalDue = $state("");
let modalTags = $state("");
let modalStatus = $state("todo");

let expandedId = $state<string | null>(null);
let dragOverCol = $state<string | null>(null);
let draggingId = $state<string | null>(null);
let dragGhost: HTMLElement | null = null;

onMount(() => {
	taskStore.load();
});

function openAdd() {
	modalMode = "add";
	modalId = "";
	modalTitle = "";
	modalDescription = "";
	modalPriority = "";
	modalDue = "";
	modalTags = "";
	modalStatus = "todo";
	modalOpen = true;
}

function openEdit(task: Task) {
	modalMode = "edit";
	modalId = task.id;
	modalTitle = task.title;
	modalDescription = task.description ?? "";
	modalPriority = task.priority ?? "";
	modalDue = task.due ?? "";
	modalTags = task.tags?.join(", ") ?? "";
	modalStatus = task.status;
	modalOpen = true;
}

function closeModal() {
	modalOpen = false;
}

function parseTags(raw: string): string[] | undefined {
	const tags = raw
		.split(",")
		.map((t) => t.trim())
		.filter(Boolean);
	return tags.length > 0 ? tags : undefined;
}

async function handleModalSubmit(e: SubmitEvent) {
	e.preventDefault();
	if (!modalTitle.trim()) return;

	const data: Partial<Task> = {
		title: modalTitle.trim(),
		description: modalDescription.trim() || undefined,
		priority: modalPriority || undefined,
		due: modalDue || undefined,
		tags: parseTags(modalTags),
	};

	if (modalMode === "add") {
		data.status = modalStatus;
		await taskStore.add(data);
	} else {
		data.status = modalStatus;
		await taskStore.update(modalId, data);
	}
	closeModal();
}

function priorityColor(p?: string): string {
	if (p === "high") return "text-error";
	if (p === "medium") return "text-warning";
	return "text-text-muted";
}

function priorityLabel(p?: string): string {
	if (p === "high") return "!";
	if (p === "medium") return "!!";
	if (p === "low") return "!!!";
	return "";
}

function isOverdue(due?: string): boolean {
	if (!due) return false;
	return due < new Date().toISOString().slice(0, 10);
}

function formatDate(due: string): string {
	const today = new Date().toISOString().slice(0, 10);
	const tomorrow = new Date(Date.now() + 86400000).toISOString().slice(0, 10);
	if (due === today) return "Today";
	if (due === tomorrow) return "Tomorrow";
	const d = new Date(due + "T00:00:00");
	return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

async function handleDrop(e: DragEvent, status: string) {
	e.preventDefault();
	dragOverCol = null;
	const id = e.dataTransfer?.getData("text/plain");
	if (id) {
		await taskStore.moveStatus(id, status);
	}
}

function handleDragStart(e: DragEvent, id: string) {
	if (!e.dataTransfer) return;
	e.dataTransfer.setData("text/plain", id);
	e.dataTransfer.effectAllowed = "move";
	draggingId = id;

	const el = e.currentTarget as HTMLElement;
	const rect = el.getBoundingClientRect();

	// Create a tilted ghost clone
	const ghost = el.cloneNode(true) as HTMLElement;
	ghost.style.position = "fixed";
	ghost.style.top = "-9999px";
	ghost.style.left = "-9999px";
	ghost.style.width = `${rect.width}px`;
	ghost.style.transform = "rotate(-3deg) scale(1.04)";
	ghost.style.opacity = "0.92";
	ghost.style.pointerEvents = "none";
	ghost.style.boxShadow = "0 8px 24px rgba(0,0,0,0.25)";
	ghost.style.borderRadius = "8px";
	document.body.appendChild(ghost);
	dragGhost = ghost;

	const offsetX = e.clientX - rect.left;
	const offsetY = e.clientY - rect.top;
	e.dataTransfer.setDragImage(ghost, offsetX, offsetY);
}

function handleDragEnd() {
	draggingId = null;
	if (dragGhost) {
		dragGhost.remove();
		dragGhost = null;
	}
}

function handleDragOver(e: DragEvent, col: string) {
	e.preventDefault();
	if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
	dragOverCol = col;
}

function handleDragLeave(e: DragEvent, col: string) {
	const target = e.currentTarget as HTMLElement;
	const related = e.relatedTarget as Node | null;
	if (related && target.contains(related)) return;
	if (dragOverCol === col) dragOverCol = null;
}

function handleKeydown(e: KeyboardEvent) {
	if (e.key === "Escape" && modalOpen) {
		closeModal();
	}
}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="flex h-full flex-col">
	<!-- Header -->
	<div class="flex shrink-0 items-center gap-2 border-b border-border px-4 py-2.5">
		<h1 class="text-[13px] font-medium text-text-primary">Tasks</h1>

		<div class="ml-auto flex items-center gap-1.5">
			<!-- Search -->
			<div class="relative">
				<Icon
					src={FiSearch}
					size="13"
					className="pointer-events-none absolute left-2 top-1/2 -translate-y-1/2 text-text-muted"
				/>
				<input
					type="text"
					placeholder="Search..."
					bind:value={taskStore.search}
					class="h-7 w-40 rounded-md border border-border bg-bg-tertiary pl-7 pr-2 text-[12px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
				/>
			</div>

			<!-- Status filter -->
			<select
				bind:value={taskStore.filterStatus}
				class="h-7 rounded-md border border-border bg-bg-tertiary px-2 text-[12px] text-text-secondary outline-none focus:border-accent"
			>
				<option value="">All status</option>
				<option value="todo">To Do</option>
				<option value="doing">In Progress</option>
				<option value="done">Done</option>
			</select>

			<!-- Tag filter -->
			{#if taskStore.allTags.length > 0}
				<select
					bind:value={taskStore.filterTag}
					class="h-7 rounded-md border border-border bg-bg-tertiary px-2 text-[12px] text-text-secondary outline-none focus:border-accent"
				>
					<option value="">All tags</option>
					{#each taskStore.allTags as tag}
						<option value={tag}>{tag}</option>
					{/each}
				</select>
			{/if}

			<!-- Due filter -->
			<input
				type="date"
				bind:value={taskStore.filterDue}
				class="h-7 rounded-md border border-border bg-bg-tertiary px-2 text-[12px] text-text-secondary outline-none focus:border-accent"
			/>
			{#if taskStore.filterDue}
				<button
					onclick={() => (taskStore.filterDue = "")}
					class="flex h-7 w-7 items-center justify-center rounded-md text-text-muted hover:bg-overlay-light hover:text-text-secondary"
					title="Clear date filter"
				>
					<Icon src={FiX} size="13" />
				</button>
			{/if}

			<!-- View toggle -->
			<div class="ml-1 flex rounded-md border border-border">
				<button
					onclick={() => (taskStore.view = "list")}
					class="flex h-7 w-7 items-center justify-center rounded-l-md text-[12px] transition-colors duration-100 {taskStore.view === 'list' ? 'bg-overlay-medium text-text-primary' : 'text-text-muted hover:bg-overlay-light hover:text-text-secondary'}"
					title="List view"
				>
					<Icon src={FiList} size="14" />
				</button>
				<button
					onclick={() => (taskStore.view = "kanban")}
					class="flex h-7 w-7 items-center justify-center rounded-r-md text-[12px] transition-colors duration-100 {taskStore.view === 'kanban' ? 'bg-overlay-medium text-text-primary' : 'text-text-muted hover:bg-overlay-light hover:text-text-secondary'}"
					title="Kanban view"
				>
					<Icon src={FiColumns} size="14" />
				</button>
			</div>

			<!-- Add button -->
			<button
				onclick={openAdd}
				class="flex h-7 items-center gap-1 rounded-md bg-text-primary px-2.5 text-[12px] font-medium text-bg transition-opacity duration-100 hover:opacity-80"
			>
				<Icon src={FiPlus} size="13" />
				<span class="hidden sm:inline">Add</span>
			</button>
		</div>
	</div>

	<!-- Content -->
	{#if taskStore.loading}
		<div class="flex flex-1 items-center justify-center">
			<span class="text-[13px] text-text-muted">Loading tasks...</span>
		</div>
	{:else if taskStore.view === "list"}
		<!-- List View -->
		<div class="flex-1 overflow-y-auto">
			{#if taskStore.filtered.length === 0}
				<div class="flex h-full items-center justify-center">
					<span class="text-[13px] text-text-muted">No tasks found.</span>
				</div>
			{:else}
				<div class="flex flex-col">
					{#each taskStore.filtered as task (task.id)}
						<!-- Task row -->
						<div class="group flex items-center gap-2.5 border-b border-border px-4 py-2 transition-colors duration-75 hover:bg-overlay-subtle">
							<!-- Status checkbox -->
							<button
								onclick={() =>
									task.status === "done"
										? taskStore.update(task.id, { status: "todo" })
										: taskStore.complete(task.id)}
								class="flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded border transition-colors duration-100 {task.status === 'done' ? 'border-success bg-success/20 text-success' : 'border-border-light text-transparent hover:border-text-muted hover:text-text-muted'}"
							>
								<Icon src={FiCheck} size="11" />
							</button>

							<!-- Content -->
							<button
								class="flex min-w-0 flex-1 items-center gap-2 text-left"
								onclick={() => (expandedId = expandedId === task.id ? null : task.id)}
							>
								<span
									class="truncate text-[13px] {task.status === 'done' ? 'text-text-muted line-through' : 'text-text-primary'}"
								>
									{task.title}
								</span>
								{#if task.priority}
									<span class="shrink-0 text-[11px] font-bold {priorityColor(task.priority)}" title="{task.priority} priority">
										{priorityLabel(task.priority)}
									</span>
								{/if}
								{#if task.status === "doing"}
									<span class="shrink-0 rounded-full bg-accent/15 px-1.5 py-0.5 text-[10px] font-medium text-accent">
										In Progress
									</span>
								{/if}
								{#if task.due}
									<span class="shrink-0 flex items-center gap-0.5 text-[11px] {isOverdue(task.due) && task.status !== 'done' ? 'text-error' : 'text-text-muted'}">
										<Icon src={FiCalendar} size="10" />
										{formatDate(task.due)}
									</span>
								{/if}
								{#if task.tags && task.tags.length > 0}
									{#each task.tags as tag}
										<span class="shrink-0 flex items-center gap-0.5 text-[11px] text-text-muted">
											<Icon src={FiTag} size="9" />
											{tag}
										</span>
									{/each}
								{/if}
							</button>

							<!-- Actions -->
							<div class="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity duration-100 group-hover:opacity-100">
								{#if task.status !== "done"}
									{#if task.status === "todo"}
										<button
											onclick={() => taskStore.update(task.id, { status: "doing" })}
											class="rounded px-1.5 py-0.5 text-[11px] text-text-muted hover:bg-overlay-light hover:text-accent"
											title="Start"
										>
											Start
										</button>
									{:else}
										<button
											onclick={() => taskStore.update(task.id, { status: "todo" })}
											class="rounded px-1.5 py-0.5 text-[11px] text-text-muted hover:bg-overlay-light hover:text-text-secondary"
											title="Move to To Do"
										>
											Pause
										</button>
									{/if}
								{/if}
								<button
									onclick={() => openEdit(task)}
									class="rounded px-1.5 py-0.5 text-[11px] text-text-muted hover:bg-overlay-light hover:text-text-secondary"
									title="Edit"
								>
									Edit
								</button>
								<button
									onclick={() => taskStore.remove(task.id)}
									class="rounded p-1 text-text-muted hover:bg-overlay-light hover:text-danger"
									title="Delete"
								>
									<Icon src={FiTrash2} size="12" />
								</button>
							</div>

							<!-- Expand arrow -->
							{#if task.description}
								<button
									onclick={() => (expandedId = expandedId === task.id ? null : task.id)}
									class="shrink-0 text-text-muted"
								>
									<Icon src={expandedId === task.id ? FiChevronUp : FiChevronDown} size="13" />
								</button>
							{/if}
						</div>

						<!-- Expanded description -->
						{#if expandedId === task.id && task.description}
							<div class="border-b border-border bg-overlay-subtle px-4 py-2 pl-11">
								<p class="whitespace-pre-wrap text-[12px] leading-relaxed text-text-secondary">
									{task.description}
								</p>
							</div>
						{/if}
					{/each}
				</div>
			{/if}
		</div>
	{:else}
		<!-- Kanban View -->
		<div class="flex flex-1 gap-3 overflow-x-auto p-3">
			{#each [
				{ key: "todo", label: "To Do", color: "text-text-secondary" },
				{ key: "doing", label: "In Progress", color: "text-accent" },
				{ key: "done", label: "Done", color: "text-success" },
			] as col (col.key)}
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div
					class="flex w-64 shrink-0 flex-col rounded-lg border bg-bg-secondary transition-colors duration-100 {dragOverCol === col.key ? 'border-accent bg-accent/5' : 'border-border'}"
					role="group"
					aria-label="{col.label} column"
					ondrop={(e) => handleDrop(e, col.key)}
					ondragover={(e) => handleDragOver(e, col.key)}
					ondragleave={(e) => handleDragLeave(e, col.key)}
				>
					<!-- Column header -->
					<div class="flex items-center gap-2 px-3 py-2.5 border-b border-border">
						<span class="text-[12px] font-medium {col.color}">{col.label}</span>
						<span class="text-[11px] text-text-muted">
							{taskStore.kanbanColumns[col.key as keyof typeof taskStore.kanbanColumns].length}
						</span>
					</div>

					<!-- Cards -->
					<div class="flex flex-1 flex-col gap-1.5 overflow-y-auto p-1.5">
						{#each taskStore.kanbanColumns[col.key as keyof typeof taskStore.kanbanColumns] as task (task.id)}
							<!-- svelte-ignore a11y_no_static_element_interactions -->
							<div
								draggable="true"
								role="listitem"
								ondragstart={(e) => handleDragStart(e, task.id)}
								ondragend={handleDragEnd}
								class="group flex flex-col gap-1 rounded-md border border-border bg-bg p-2.5 transition-[border-color,box-shadow,transform,opacity] duration-150 hover:border-border-light cursor-grab active:cursor-grabbing {draggingId === task.id ? 'rotate-[-2deg] scale-[0.97] opacity-40' : ''}"
							>
								<div class="flex items-start justify-between gap-1.5">
									<span class="text-[12px] leading-snug text-text-primary {task.status === 'done' ? 'line-through text-text-muted' : ''}">{task.title}</span>
									{#if task.priority}
										<span class="shrink-0 text-[11px] font-bold {priorityColor(task.priority)}" title="{task.priority} priority">
											{priorityLabel(task.priority)}
										</span>
									{/if}
								</div>
								{#if task.description}
									<p class="text-[11px] leading-snug text-text-muted line-clamp-2">{task.description}</p>
								{/if}
								<div class="flex items-center gap-1.5 mt-0.5">
									{#if task.due}
										<span class="flex items-center gap-0.5 text-[10px] {isOverdue(task.due) && task.status !== 'done' ? 'text-error' : 'text-text-muted'}">
											<Icon src={FiCalendar} size="9" />
											{formatDate(task.due)}
										</span>
									{/if}
									{#if task.tags && task.tags.length > 0}
										{#each task.tags as tag}
											<span class="rounded-full bg-overlay-light px-1.5 py-0.5 text-[10px] text-text-muted">{tag}</span>
										{/each}
									{/if}
								</div>
								<!-- Card actions -->
								<div class="flex items-center gap-0.5 mt-0.5 opacity-0 transition-opacity duration-100 group-hover:opacity-100">
									{#if task.status !== "done"}
										<button
											onclick={() => taskStore.complete(task.id)}
											class="rounded p-0.5 text-text-muted hover:text-success"
											title="Complete"
										>
											<Icon src={FiCheck} size="12" />
										</button>
									{/if}
									<button
										onclick={() => openEdit(task)}
										class="rounded px-1 py-0.5 text-[10px] text-text-muted hover:text-text-secondary"
									>
										Edit
									</button>
									<button
										onclick={() => taskStore.remove(task.id)}
										class="ml-auto rounded p-0.5 text-text-muted hover:text-danger"
										title="Delete"
									>
										<Icon src={FiTrash2} size="11" />
									</button>
								</div>
							</div>
						{/each}
						{#if taskStore.kanbanColumns[col.key as keyof typeof taskStore.kanbanColumns].length === 0}
							<div class="flex items-center justify-center py-6 text-[12px] text-text-muted">
								{dragOverCol === col.key ? "Drop here" : "No tasks"}
							</div>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Task Modal -->
{#if modalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center">
		<!-- Backdrop -->
		<button
			class="absolute inset-0 bg-black/50 backdrop-blur-sm"
			onclick={closeModal}
			tabindex="-1"
			aria-label="Close modal"
		></button>

		<!-- Dialog -->
		<form
			onsubmit={handleModalSubmit}
			class="relative z-10 flex w-full max-w-md flex-col gap-3 rounded-xl border border-border bg-bg-secondary p-5 shadow-elevated"
		>
			<div class="flex items-center justify-between">
				<h2 class="text-[14px] font-medium text-text-primary">
					{modalMode === "add" ? "New Task" : "Edit Task"}
				</h2>
				<button
					type="button"
					onclick={closeModal}
					class="flex h-6 w-6 items-center justify-center rounded text-text-muted hover:bg-overlay-light hover:text-text-secondary"
				>
					<Icon src={FiX} size="14" />
				</button>
			</div>

			<!-- Title -->
			<input
				type="text"
				bind:value={modalTitle}
				placeholder="Task title..."
				class="rounded-md border border-border bg-bg-tertiary px-3 py-2 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
			/>

			<!-- Description -->
			<textarea
				bind:value={modalDescription}
				placeholder="Description (optional)"
				rows="3"
				class="resize-none rounded-md border border-border bg-bg-tertiary px-3 py-2 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
			></textarea>

			<!-- Row: Status + Priority -->
			<div class="flex gap-2">
				<label class="flex min-w-0 flex-1 flex-col gap-1">
					<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Status</span>
					<select
						bind:value={modalStatus}
						class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary outline-none focus:border-accent"
					>
						<option value="todo">To Do</option>
						<option value="doing">In Progress</option>
						<option value="done">Done</option>
					</select>
				</label>
				<label class="flex min-w-0 flex-1 flex-col gap-1">
					<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Priority</span>
					<select
						bind:value={modalPriority}
						class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary outline-none focus:border-accent"
					>
						<option value="">None</option>
						<option value="low">Low</option>
						<option value="medium">Medium</option>
						<option value="high">High</option>
					</select>
				</label>
			</div>

			<!-- Row: Due + Tags -->
			<div class="flex gap-2">
				<label class="flex min-w-0 flex-1 flex-col gap-1">
					<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Due Date</span>
					<input
						type="date"
						bind:value={modalDue}
						class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary outline-none focus:border-accent"
					/>
				</label>
				<label class="flex min-w-0 flex-1 flex-col gap-1">
					<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Tags</span>
					<input
						type="text"
						bind:value={modalTags}
						placeholder="work, personal"
						class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
					/>
				</label>
			</div>

			<!-- Actions -->
			<div class="flex justify-end gap-2 pt-1">
				<button
					type="button"
					onclick={closeModal}
					class="rounded-md px-3 py-1.5 text-[13px] text-text-muted hover:bg-overlay-light hover:text-text-secondary"
				>
					Cancel
				</button>
				<button
					type="submit"
					disabled={!modalTitle.trim()}
					class="rounded-md bg-text-primary px-4 py-1.5 text-[13px] font-medium text-bg transition-opacity duration-100 hover:opacity-80 disabled:opacity-40"
				>
					{modalMode === "add" ? "Add Task" : "Save"}
				</button>
			</div>
		</form>
	</div>
{/if}

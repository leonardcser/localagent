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
	FiCalendar,
	FiTag,
	FiArrowLeft,
} from "svelte-icons-pack/fi";

// Detail panel state
let panelOpen = $state(false);
let panelMode = $state<"add" | "edit">("add");
let panelTitle = $state("");
let panelDescription = $state("");
let panelPriority = $state("");
let panelDue = $state("");
let panelTags = $state("");
let panelStatus = $state("todo");

let dragOverCol = $state<string | null>(null);
let draggingId = $state<string | null>(null);
let dragGhost: HTMLElement | null = null;

// Swipe state (mobile)
let swipeTaskId = $state<string | null>(null);
let swipeX = $state(0);
let swipeStartX = 0;
let swipeStartY = 0;
let swipeDirection: "horizontal" | "vertical" | null = null;
const SWIPE_THRESHOLD = 80;
const SWIPE_MAX = 120;

// Keyboard nav: dd delete sequence
let lastKeyD = 0;

onMount(async () => {
	await taskStore.load();
	if (taskStore.selectedId) {
		const task = taskStore.tasks.find((t) => t.id === taskStore.selectedId);
		if (task) openDetail(task);
	}
});

function handleTouchStart(e: TouchEvent, id: string) {
	swipeTaskId = id;
	swipeX = 0;
	swipeDirection = null;
	swipeStartX = e.touches[0].clientX;
	swipeStartY = e.touches[0].clientY;
}

function handleTouchMove(e: TouchEvent) {
	if (!swipeTaskId) return;
	const dx = e.touches[0].clientX - swipeStartX;
	const dy = e.touches[0].clientY - swipeStartY;

	if (!swipeDirection) {
		if (Math.abs(dx) > 8 || Math.abs(dy) > 8) {
			swipeDirection = Math.abs(dx) > Math.abs(dy) ? "horizontal" : "vertical";
		}
		return;
	}

	if (swipeDirection === "vertical") return;

	e.preventDefault();
	swipeX = Math.max(-SWIPE_MAX, Math.min(SWIPE_MAX, dx));
}

async function handleTouchEnd() {
	if (!swipeTaskId || swipeDirection !== "horizontal") {
		swipeTaskId = null;
		swipeX = 0;
		return;
	}

	const id = swipeTaskId;
	if (swipeX >= SWIPE_THRESHOLD) {
		swipeX = 0;
		swipeTaskId = null;
		await taskStore.complete(id);
	} else if (swipeX <= -SWIPE_THRESHOLD) {
		swipeX = 0;
		swipeTaskId = null;
		await taskStore.remove(id);
		if (panelOpen && taskStore.selectedId === id) closePanel();
	} else {
		swipeX = 0;
		swipeTaskId = null;
	}
}

function openAdd() {
	panelMode = "add";
	taskStore.selectedId = "";
	panelTitle = "";
	panelDescription = "";
	panelPriority = "";
	panelDue = "";
	panelTags = "";
	panelStatus = "todo";
	panelOpen = true;
}

function openDetail(task: Task) {
	panelMode = "edit";
	taskStore.selectedId = task.id;
	panelTitle = task.title;
	panelDescription = task.description ?? "";
	panelPriority = task.priority ?? "";
	panelDue = task.due ?? "";
	panelTags = task.tags?.join(", ") ?? "";
	panelStatus = task.status;
	panelOpen = true;
}

function closePanel() {
	panelOpen = false;
}

function navigateTask(direction: -1 | 1) {
	const list = taskStore.filtered;
	if (list.length === 0) return;

	if (!panelOpen || panelMode !== "edit") {
		openDetail(list[0]);
		return;
	}

	const idx = list.findIndex((t) => t.id === taskStore.selectedId);
	const next = idx + direction;
	if (next >= 0 && next < list.length) {
		openDetail(list[next]);
	}
}

function parseTags(raw: string): string[] | undefined {
	const tags = raw
		.split(",")
		.map((t) => t.trim())
		.filter(Boolean);
	return tags.length > 0 ? tags : undefined;
}

async function handleSubmit(e: SubmitEvent) {
	e.preventDefault();
	if (!panelTitle.trim()) return;

	const data: Partial<Task> = {
		title: panelTitle.trim(),
		description: panelDescription.trim() || undefined,
		priority: panelPriority || undefined,
		due: panelDue || undefined,
		tags: parseTags(panelTags),
		status: panelStatus,
	};

	if (panelMode === "add") {
		await taskStore.add(data);
	} else {
		await taskStore.update(taskStore.selectedId, data);
	}
	closePanel();
}

async function handleDelete() {
	if (panelMode === "edit" && taskStore.selectedId) {
		await taskStore.remove(taskStore.selectedId);
		closePanel();
	}
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

function isInputFocused(e: KeyboardEvent): boolean {
	const tag = (e.target as HTMLElement)?.tagName;
	return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT";
}

function handleKeydown(e: KeyboardEvent) {
	if (e.key === "Escape" && panelOpen) {
		closePanel();
		return;
	}

	if (isInputFocused(e)) return;

	// Delete / Backspace: delete selected task
	if (
		(e.key === "Delete" || e.key === "Backspace") &&
		panelOpen &&
		panelMode === "edit" &&
		taskStore.selectedId
	) {
		e.preventDefault();
		handleDelete();
		return;
	}

	// Arrow keys / j / k: navigate tasks
	if (e.key === "ArrowDown" || e.key === "j") {
		e.preventDefault();
		navigateTask(1);
		return;
	}
	if (e.key === "ArrowUp" || e.key === "k") {
		e.preventDefault();
		navigateTask(-1);
		return;
	}

	// dd: delete (vim-style)
	if (
		e.key === "d" &&
		panelOpen &&
		panelMode === "edit" &&
		taskStore.selectedId
	) {
		const now = Date.now();
		if (now - lastKeyD < 400) {
			lastKeyD = 0;
			e.preventDefault();
			handleDelete();
			return;
		}
		lastKeyD = now;
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

	<!-- Content + Detail Panel -->
	<div class="flex flex-1 overflow-hidden">
		<!-- Main content -->
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
							<div class="relative overflow-hidden border-b border-border">
								{#if swipeTaskId === task.id && swipeX > 0}
									<div class="absolute inset-0 flex items-center bg-success/15 px-4 text-success">
										<Icon src={FiCheck} size="16" />
										<span class="ml-2 text-[12px] font-medium">Complete</span>
									</div>
								{/if}
								{#if swipeTaskId === task.id && swipeX < 0}
									<div class="absolute inset-0 flex items-center justify-end bg-danger/15 px-4 text-danger">
										<span class="mr-2 text-[12px] font-medium">Delete</span>
										<Icon src={FiTrash2} size="16" />
									</div>
								{/if}

								<button
									class="relative flex w-full items-center gap-2.5 border-l-2 bg-bg px-4 py-2 text-left transition-colors duration-75 hover:bg-overlay-subtle {panelOpen && taskStore.selectedId === task.id ? 'border-l-accent bg-overlay-subtle' : 'border-l-transparent'}"
									style={swipeTaskId === task.id && swipeX !== 0 ? `transform: translateX(${swipeX}px)` : ""}
									onclick={() => openDetail(task)}
									ontouchstart={(e) => handleTouchStart(e, task.id)}
									ontouchmove={handleTouchMove}
									ontouchend={handleTouchEnd}
								>
									<span
										role="checkbox"
										tabindex="-1"
										aria-checked={task.status === "done"}
										onclick={(e) => {
											e.stopPropagation();
											task.status === "done"
												? taskStore.update(task.id, { status: "todo" })
												: taskStore.complete(task.id);
										}}
										onkeydown={(e) => {
											if (e.key === "Enter" || e.key === " ") {
												e.preventDefault();
												e.stopPropagation();
												task.status === "done"
													? taskStore.update(task.id, { status: "todo" })
													: taskStore.complete(task.id);
											}
										}}
										class="flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded border transition-colors duration-100 {task.status === 'done' ? 'border-success bg-success/20 text-success' : 'border-border-light text-transparent hover:border-text-muted hover:text-text-muted'}"
									>
										<Icon src={FiCheck} size="11" />
									</span>

									<span class="flex min-w-0 flex-1 items-center gap-2">
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
									</span>
								</button>
							</div>
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
								<div
									draggable="true"
									role="button"
									tabindex="0"
									ondragstart={(e) => handleDragStart(e, task.id)}
									ondragend={handleDragEnd}
									onclick={() => openDetail(task)}
									onkeydown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); openDetail(task); } }}
									class="group flex cursor-pointer flex-col gap-1 rounded-md border-l-2 border bg-bg p-2.5 transition-[border-color,box-shadow,transform,opacity] duration-150 hover:border-border-light {panelOpen && taskStore.selectedId === task.id && draggingId !== task.id ? 'border-accent border-l-accent' : 'border-border border-l-transparent'} {draggingId === task.id ? 'rotate-[-2deg] scale-[0.97] cursor-grabbing opacity-40' : ''}"
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

		<!-- Desktop detail sidebar -->
		{#if panelOpen}
			<div class="hidden w-80 shrink-0 flex-col border-l border-border bg-bg-secondary md:flex">
				<form onsubmit={handleSubmit} class="flex flex-1 flex-col overflow-hidden">
					<!-- Header -->
					<div class="flex items-center justify-between border-b border-border px-4 py-2.5">
						<h2 class="text-[13px] font-medium text-text-primary">
							{panelMode === "add" ? "New Task" : "Edit Task"}
						</h2>
						<button
							type="button"
							onclick={closePanel}
							class="flex h-6 w-6 items-center justify-center rounded text-text-muted hover:bg-overlay-light hover:text-text-secondary"
						>
							<Icon src={FiX} size="14" />
						</button>
					</div>

					<!-- Fields -->
					<div class="flex flex-1 flex-col gap-3 overflow-y-auto p-4">
						<label class="flex flex-col gap-1">
							<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Title</span>
							<input
								type="text"
								bind:value={panelTitle}
								placeholder="Task title..."
								class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
							/>
						</label>

						<label class="flex flex-col gap-1">
							<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Description</span>
							<textarea
								bind:value={panelDescription}
								placeholder="Description (optional)"
								rows="4"
								class="resize-none rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
							></textarea>
						</label>

						<label class="flex flex-col gap-1">
							<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Status</span>
							<select
								bind:value={panelStatus}
								class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary outline-none focus:border-accent"
							>
								<option value="todo">To Do</option>
								<option value="doing">In Progress</option>
								<option value="done">Done</option>
							</select>
						</label>

						<label class="flex flex-col gap-1">
							<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Priority</span>
							<select
								bind:value={panelPriority}
								class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary outline-none focus:border-accent"
							>
								<option value="">None</option>
								<option value="low">Low</option>
								<option value="medium">Medium</option>
								<option value="high">High</option>
							</select>
						</label>

						<label class="flex flex-col gap-1">
							<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Due Date</span>
							<input
								type="date"
								bind:value={panelDue}
								class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary outline-none focus:border-accent"
							/>
						</label>

						<label class="flex flex-col gap-1">
							<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Tags</span>
							<input
								type="text"
								bind:value={panelTags}
								placeholder="work, personal"
								class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
							/>
						</label>
					</div>

					<!-- Actions -->
					<div class="flex items-center gap-2 border-t border-border px-4 py-3">
						{#if panelMode === "edit"}
							<button
								type="button"
								onclick={handleDelete}
								class="flex h-7 w-7 items-center justify-center rounded-md text-text-muted hover:bg-overlay-light hover:text-danger"
								title="Delete task"
							>
								<Icon src={FiTrash2} size="14" />
							</button>
						{/if}
						<div class="ml-auto flex items-center gap-2">
							<button
								type="button"
								onclick={closePanel}
								class="rounded-md px-3 py-1.5 text-[12px] text-text-muted hover:bg-overlay-light hover:text-text-secondary"
							>
								Cancel
							</button>
							<button
								type="submit"
								disabled={!panelTitle.trim()}
								class="rounded-md bg-text-primary px-3 py-1.5 text-[12px] font-medium text-bg transition-opacity duration-100 hover:opacity-80 disabled:opacity-40"
							>
								{panelMode === "add" ? "Add" : "Save"}
							</button>
						</div>
					</div>
				</form>
			</div>
		{/if}
	</div>
</div>

<!-- Mobile detail panel (full screen) -->
{#if panelOpen}
	<div class="fixed inset-0 z-40 flex flex-col bg-bg md:hidden">
		<form onsubmit={handleSubmit} class="flex flex-1 flex-col overflow-hidden">
			<!-- Mobile header -->
			<div
				class="flex items-center gap-2 border-b border-border px-3 py-2.5"
				style="padding-top: max(env(safe-area-inset-top, 0px), 10px)"
			>
				<button
					type="button"
					onclick={closePanel}
					class="flex h-8 w-8 items-center justify-center rounded-md text-text-secondary hover:bg-overlay-light"
				>
					<Icon src={FiArrowLeft} size="18" />
				</button>
				<h2 class="text-[13px] font-medium text-text-primary">
					{panelMode === "add" ? "New Task" : "Edit Task"}
				</h2>
				{#if panelMode === "edit"}
					<button
						type="button"
						onclick={handleDelete}
						class="ml-auto flex h-8 w-8 items-center justify-center rounded-md text-text-muted hover:bg-overlay-light hover:text-danger"
						title="Delete task"
					>
						<Icon src={FiTrash2} size="16" />
					</button>
				{/if}
			</div>

			<!-- Fields -->
			<div class="flex flex-1 flex-col gap-3 overflow-y-auto p-4">
				<label class="flex flex-col gap-1">
					<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Title</span>
					<input
						type="text"
						bind:value={panelTitle}
						placeholder="Task title..."
						class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
					/>
				</label>

				<label class="flex flex-col gap-1">
					<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Description</span>
					<textarea
						bind:value={panelDescription}
						placeholder="Description (optional)"
						rows="4"
						class="resize-none rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
					></textarea>
				</label>

				<label class="flex flex-col gap-1">
					<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Status</span>
					<select
						bind:value={panelStatus}
						class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary outline-none focus:border-accent"
					>
						<option value="todo">To Do</option>
						<option value="doing">In Progress</option>
						<option value="done">Done</option>
					</select>
				</label>

				<label class="flex flex-col gap-1">
					<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Priority</span>
					<select
						bind:value={panelPriority}
						class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary outline-none focus:border-accent"
					>
						<option value="">None</option>
						<option value="low">Low</option>
						<option value="medium">Medium</option>
						<option value="high">High</option>
					</select>
				</label>

				<label class="flex flex-col gap-1">
					<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Due Date</span>
					<input
						type="date"
						bind:value={panelDue}
						class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary outline-none focus:border-accent"
					/>
				</label>

				<label class="flex flex-col gap-1">
					<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Tags</span>
					<input
						type="text"
						bind:value={panelTags}
						placeholder="work, personal"
						class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
					/>
				</label>
			</div>

			<!-- Actions -->
			<div
				class="flex items-center justify-end gap-2 border-t border-border px-4 py-3"
				style="padding-bottom: max(env(safe-area-inset-bottom, 0px), 12px)"
			>
				<button
					type="button"
					onclick={closePanel}
					class="rounded-md px-3 py-1.5 text-[13px] text-text-muted hover:bg-overlay-light hover:text-text-secondary"
				>
					Cancel
				</button>
				<button
					type="submit"
					disabled={!panelTitle.trim()}
					class="rounded-md bg-text-primary px-4 py-1.5 text-[13px] font-medium text-bg transition-opacity duration-100 hover:opacity-80 disabled:opacity-40"
				>
					{panelMode === "add" ? "Add Task" : "Save"}
				</button>
			</div>
		</form>
	</div>
{/if}

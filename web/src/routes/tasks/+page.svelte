<script lang="ts">
import { onMount } from "svelte";
import { taskStore, type SmartList } from "$lib/stores/task.svelte";
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
	FiSun,
	FiInbox,
	FiChevronRight,
} from "svelte-icons-pack/fi";

let panelOpen = $state(false);
let panelMode = $state<"add" | "edit">("add");
let panelTitle = $state("");
let panelDescription = $state("");
let panelPriority = $state("");
let panelDue = $state("");
let panelTags = $state("");
let panelStatus = $state("todo");

let quickAddValue = $state("");
let quickAddFocused = $state(false);

let showSidebar = $state(false);
let showSearch = $state(false);

let dragOverCol = $state<string | null>(null);
let draggingId = $state<string | null>(null);
let dragGhost: HTMLElement | null = null;

let swipeTaskId = $state<string | null>(null);
let swipeX = $state(0);
let swipeStartX = 0;
let swipeStartY = 0;
let swipeDirection: "horizontal" | "vertical" | null = null;
const SWIPE_THRESHOLD = 80;
const SWIPE_MAX = 120;

let lastKeyD = 0;

onMount(async () => {
	await taskStore.load();
	if (taskStore.selectedId) {
		const task = taskStore.tasks.find((t) => t.id === taskStore.selectedId);
		if (task) openDetail(task);
	}
});

const smartLists: { key: SmartList; label: string; icon: typeof FiSun }[] = [
	{ key: "all", label: "All", icon: FiList },
	{ key: "today", label: "Today", icon: FiSun },
	{ key: "tomorrow", label: "Tomorrow", icon: FiCalendar },
	{ key: "next7", label: "Next 7 Days", icon: FiCalendar },
	{ key: "inbox", label: "Inbox", icon: FiInbox },
	{ key: "done", label: "Completed", icon: FiCheck },
];

function smartListLabel(): string {
	return smartLists.find((s) => s.key === taskStore.smartList)?.label ?? "All";
}

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

async function handleQuickAdd(e: KeyboardEvent) {
	if (e.key !== "Enter" || !quickAddValue.trim()) return;
	const due =
		taskStore.smartList === "today"
			? new Date().toISOString().slice(0, 10)
			: taskStore.smartList === "tomorrow"
				? new Date(Date.now() + 86400000).toISOString().slice(0, 10)
				: undefined;
	await taskStore.add({
		title: quickAddValue.trim(),
		status: "todo",
		due,
	});
	quickAddValue = "";
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
	ghost.style.border = "1px solid var(--color-border-light)";
	ghost.style.borderLeft = "2px solid transparent";
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
	if (e.key === "Escape") {
		if (panelOpen) {
			closePanel();
			return;
		}
		if (showSearch) {
			showSearch = false;
			taskStore.search = "";
			return;
		}
	}

	if (isInputFocused(e)) return;

	if (e.key === "/" && !showSearch) {
		e.preventDefault();
		showSearch = true;
		return;
	}

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

<div class="flex h-full">
	<!-- Desktop sidebar: smart lists -->
	<div class="hidden w-52 shrink-0 flex-col border-r border-border bg-bg md:flex">
		<!-- Search toggle -->
		<div class="px-3 pt-3 pb-1">
			{#if showSearch}
				<div class="relative">
					<Icon
						src={FiSearch}
						size="13"
						className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-text-muted"
					/>
					<!-- svelte-ignore a11y_autofocus -->
					<input
						type="text"
						placeholder="Search tasks..."
						bind:value={taskStore.search}
						class="h-8 w-full rounded-lg border border-border bg-bg-tertiary pl-8 pr-8 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
						autofocus
					/>
					<button
						onclick={() => { showSearch = false; taskStore.search = ""; }}
						class="absolute right-1.5 top-1/2 -translate-y-1/2 flex h-5 w-5 items-center justify-center rounded text-text-muted hover:text-text-secondary"
					>
						<Icon src={FiX} size="12" />
					</button>
				</div>
			{:else}
				<button
					onclick={() => (showSearch = true)}
					class="flex h-8 w-full items-center gap-2 rounded-lg px-2.5 text-[13px] text-text-muted transition-colors hover:bg-overlay-light hover:text-text-secondary"
				>
					<Icon src={FiSearch} size="14" />
					<span>Search</span>
					<kbd class="ml-auto rounded border border-border px-1 py-0.5 text-[10px] text-text-muted">/</kbd>
				</button>
			{/if}
		</div>

		<!-- Smart lists -->
		<div class="flex flex-col px-1.5 py-2">
			<span class="px-2 pb-1 text-[10px] font-semibold uppercase tracking-widest text-text-muted">Smart Lists</span>
			{#each smartLists as item}
				<button
					onclick={() => { taskStore.smartList = item.key; taskStore.search = ""; showSearch = false; showSidebar = false; }}
					class="group flex items-center gap-2.5 rounded-lg px-2.5 py-1.75 text-[13px] transition-colors
						{taskStore.smartList === item.key && !taskStore.search ? 'bg-accent/10 text-accent font-medium' : 'text-text-secondary hover:bg-overlay-light hover:text-text-primary'}"
				>
					<Icon
						src={item.icon}
						size="15"
						className="shrink-0 {taskStore.smartList === item.key && !taskStore.search ? 'text-accent' : 'text-text-muted group-hover:text-text-secondary'}"
					/>
					<span class="flex-1 text-left">{item.label}</span>
					<span class="min-w-5 text-right text-[11px] tabular-nums {taskStore.smartList === item.key && !taskStore.search ? 'text-accent/70' : 'text-text-muted'}">
						{taskStore.counts[item.key]}
					</span>
				</button>
			{/each}
		</div>

		<!-- Tags -->
		{#if taskStore.allTags.length > 0}
			<div class="flex flex-col px-1.5 py-2 border-t border-border">
				<span class="px-2 pb-1 text-[10px] font-semibold uppercase tracking-widest text-text-muted">Tags</span>
				{#each taskStore.allTags as tag}
					<button
						onclick={() => {
							taskStore.filterTag = taskStore.filterTag === tag ? "" : tag;
							showSidebar = false;
						}}
						class="flex items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-[13px] transition-colors
							{taskStore.filterTag === tag ? 'bg-accent/10 text-accent' : 'text-text-secondary hover:bg-overlay-light hover:text-text-primary'}"
					>
						<Icon src={FiTag} size="13" className="shrink-0 {taskStore.filterTag === tag ? 'text-accent' : 'text-text-muted'}" />
						<span>{tag}</span>
					</button>
				{/each}
			</div>
		{/if}

	</div>

	<!-- Main area -->
	<div class="flex flex-1 flex-col overflow-hidden">
		<!-- Mobile header -->
		<div class="flex shrink-0 items-center gap-2 border-b border-border px-3 py-2 md:hidden">
			<button
				onclick={() => (showSidebar = !showSidebar)}
				class="flex h-8 items-center gap-1.5 rounded-lg px-2 text-[13px] font-medium text-text-primary hover:bg-overlay-light"
			>
				{smartListLabel()}
				<Icon src={FiChevronRight} size="14" className="text-text-muted rotate-90" />
			</button>
			<div class="ml-auto flex items-center gap-1">
				<button
					onclick={() => (showSearch = !showSearch)}
					class="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-text-secondary"
				>
					<Icon src={FiSearch} size="16" />
				</button>
				<button
					onclick={() => (taskStore.view = taskStore.view === "list" ? "kanban" : "list")}
					class="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-text-secondary"
				>
					<Icon src={taskStore.view === "list" ? FiColumns : FiList} size="16" />
				</button>
			</div>
		</div>

		<!-- Mobile search bar -->
		{#if showSearch}
			<div class="border-b border-border px-3 py-2 md:hidden">
				<div class="relative">
					<Icon
						src={FiSearch}
						size="14"
						className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-text-muted"
					/>
					<!-- svelte-ignore a11y_autofocus -->
					<input
						type="text"
						placeholder="Search tasks..."
						bind:value={taskStore.search}
						class="h-9 w-full rounded-lg border border-border bg-bg-tertiary pl-8 pr-8 text-[14px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
						autofocus
					/>
					<button
						onclick={() => { showSearch = false; taskStore.search = ""; }}
						class="absolute right-2 top-1/2 -translate-y-1/2 flex h-5 w-5 items-center justify-center rounded text-text-muted"
					>
						<Icon src={FiX} size="14" />
					</button>
				</div>
			</div>
		{/if}

		<!-- Desktop header -->
		<div class="hidden shrink-0 items-center gap-2 border-b border-border px-5 py-3 md:flex">
			<h1 class="text-[15px] font-semibold text-text-primary">{smartListLabel()}</h1>
			{#if taskStore.filterTag}
				<span class="flex items-center gap-1 rounded-full bg-accent/10 px-2 py-0.5 text-[11px] text-accent">
					<Icon src={FiTag} size="10" />
					{taskStore.filterTag}
					<button onclick={() => (taskStore.filterTag = "")} class="ml-0.5 hover:text-text-primary">
						<Icon src={FiX} size="10" />
					</button>
				</span>
			{/if}
			<span class="text-[12px] text-text-muted">{taskStore.filtered.length} tasks</span>
			<div class="ml-auto flex items-center gap-1.5">
				<button
					onclick={() => (taskStore.view = taskStore.view === "list" ? "kanban" : "list")}
					class="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-text-secondary"
					title={taskStore.view === "list" ? "Board view" : "List view"}
				>
					<Icon src={taskStore.view === "list" ? FiColumns : FiList} size="16" />
				</button>
				<button
					onclick={openAdd}
					class="flex h-8 items-center gap-1.5 rounded-lg bg-accent px-3 text-[13px] font-medium text-white transition-opacity hover:opacity-90"
				>
					<Icon src={FiPlus} size="14" />
					Add Task
				</button>
			</div>
		</div>

		<!-- Content -->
		<div class="flex flex-1 overflow-hidden">
			{#if taskStore.loading}
				<div class="flex flex-1 items-center justify-center">
					<span class="text-[13px] text-text-muted">Loading...</span>
				</div>
			{:else if taskStore.view === "list"}
				<div class="flex-1 overflow-y-auto">
					<!-- Quick add -->
					<div class="border-b border-border px-4 py-2 md:px-5">
						<div class="flex items-center gap-2.5">
							<div class="flex h-4.5 w-4.5 shrink-0 items-center justify-center rounded-full border border-dashed border-text-muted/40 text-text-muted/40">
								<Icon src={FiPlus} size="10" />
							</div>
							<input
								type="text"
								placeholder="Add a task..."
								bind:value={quickAddValue}
								onkeydown={handleQuickAdd}
								onfocus={() => (quickAddFocused = true)}
								onblur={() => (quickAddFocused = false)}
								class="flex-1 bg-transparent text-[13px] text-text-primary placeholder:text-text-muted/60 outline-none"
							/>
						</div>
					</div>

					{#if taskStore.filtered.length === 0}
						<div class="flex h-48 flex-col items-center justify-center gap-2">
							<span class="text-[24px] opacity-30">&#10003;</span>
							<span class="text-[13px] text-text-muted">
								{taskStore.search ? "No matching tasks" : "All clear!"}
							</span>
						</div>
					{:else}
						<div class="flex flex-col">
							{#each taskStore.filtered as task (task.id)}
								<div class="relative overflow-hidden">
									{#if swipeTaskId === task.id && swipeX > 0}
										<div class="absolute inset-0 flex items-center bg-success/15 px-4 text-success">
											<Icon src={FiCheck} size="16" />
											<span class="ml-2 text-[12px] font-medium">Done</span>
										</div>
									{/if}
									{#if swipeTaskId === task.id && swipeX < 0}
										<div class="absolute inset-0 flex items-center justify-end bg-danger/15 px-4 text-danger">
											<span class="mr-2 text-[12px] font-medium">Delete</span>
											<Icon src={FiTrash2} size="16" />
										</div>
									{/if}

									<button
										class="relative flex w-full items-center gap-3 bg-bg px-4 py-2.5 text-left transition-colors duration-75 md:px-5
											{panelOpen && taskStore.selectedId === task.id ? 'bg-accent/5' : 'hover:bg-overlay-subtle'}"
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
											class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border-[1.5px] transition-colors duration-100
												{task.status === 'done' ? 'border-success bg-success text-white' : 'border-border-light text-transparent hover:border-text-muted hover:text-text-muted/50'}"
										>
											<Icon src={FiCheck} size="11" />
										</span>

										<span class="flex min-w-0 flex-1 flex-col gap-0.5">
											<span class="flex items-center gap-2">
												<span
													class="truncate text-[13px] {task.status === 'done' ? 'text-text-muted line-through' : 'text-text-primary'}"
												>
													{task.title}
												</span>
												{#if task.status === "doing"}
													<span class="shrink-0 rounded-full bg-accent/12 px-1.5 py-0.5 text-[10px] font-medium text-accent">
														In Progress
													</span>
												{/if}
											</span>
											{#if task.description}
												<span class="truncate text-[12px] text-text-muted">{task.description}</span>
											{/if}
											<span class="flex items-center gap-2 mt-0.5">
												{#if task.due}
													<span class="flex items-center gap-1 text-[11px] {isOverdue(task.due) && task.status !== 'done' ? 'text-error' : 'text-text-muted'}">
														<Icon src={FiCalendar} size="10" />
														{formatDate(task.due)}
													</span>
												{/if}
												{#if task.tags && task.tags.length > 0}
													{#each task.tags as tag}
														<span class="flex items-center gap-0.5 text-[11px] text-text-muted">
															<Icon src={FiTag} size="9" />
															{tag}
														</span>
													{/each}
												{/if}
											</span>
										</span>

										{#if task.priority && task.status !== "done"}
											<span class="shrink-0 text-[11px] font-bold {priorityColor(task.priority)}" title="{task.priority} priority">
												{priorityLabel(task.priority)}
											</span>
										{/if}
									</button>
									<div class="mx-4 border-b border-border/50 md:mx-5"></div>
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
							class="flex w-64 shrink-0 flex-col rounded-xl border bg-bg-secondary transition-colors duration-100 {dragOverCol === col.key ? 'border-accent bg-accent/5' : 'border-border'}"
							role="group"
							aria-label="{col.label} column"
							ondrop={(e) => handleDrop(e, col.key)}
							ondragover={(e) => handleDragOver(e, col.key)}
							ondragleave={(e) => handleDragLeave(e, col.key)}
						>
							<div class="flex items-center gap-2 px-3 py-2.5 border-b border-border">
								<span class="text-[12px] font-semibold {col.color}">{col.label}</span>
								<span class="flex h-4.5 min-w-4.5 items-center justify-center rounded-full bg-overlay-light px-1 text-[10px] text-text-muted">
									{taskStore.kanbanColumns[col.key as keyof typeof taskStore.kanbanColumns].length}
								</span>
							</div>

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
										class="group flex cursor-pointer flex-col gap-1 rounded-lg border bg-bg p-2.5 transition-all duration-150 {draggingId === task.id ? 'border-border -rotate-2 scale-[0.97] cursor-grabbing opacity-40' : panelOpen && taskStore.selectedId === task.id ? 'border-accent shadow-sm' : 'border-border hover:border-border-light hover:shadow-sm'}"
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
									<div class="flex items-center justify-center py-8 text-[12px] text-text-muted">
										{dragOverCol === col.key ? "Drop here" : "No tasks"}
									</div>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			{/if}

			<!-- Desktop detail panel -->
			{#if panelOpen}
				<div class="hidden w-80 shrink-0 flex-col border-l border-border bg-bg md:flex">
					<form onsubmit={handleSubmit} class="flex flex-1 flex-col overflow-hidden">
						<div class="flex items-center justify-between border-b border-border px-4 py-3">
							<h2 class="text-[14px] font-semibold text-text-primary">
								{panelMode === "add" ? "New Task" : "Edit Task"}
							</h2>
							<div class="flex items-center gap-1">
								{#if panelMode === "edit"}
									<button
										type="button"
										onclick={handleDelete}
										class="flex h-7 w-7 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-danger"
										title="Delete task"
									>
										<Icon src={FiTrash2} size="14" />
									</button>
								{/if}
								<button
									type="button"
									onclick={closePanel}
									class="flex h-7 w-7 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-text-secondary"
								>
									<Icon src={FiX} size="14" />
								</button>
							</div>
						</div>

						<div class="flex flex-1 flex-col gap-4 overflow-y-auto p-4">
							<input
								type="text"
								bind:value={panelTitle}
								placeholder="Task title"
								class="bg-transparent text-[15px] font-medium text-text-primary placeholder:text-text-muted outline-none"
							/>

							<textarea
								bind:value={panelDescription}
								placeholder="Add notes..."
								rows="3"
								class="resize-none bg-transparent text-[13px] text-text-secondary placeholder:text-text-muted outline-none"
							></textarea>

							<div class="flex flex-col gap-3 border-t border-border pt-4">
								<div class="flex items-center justify-between">
									<span class="flex items-center gap-2 text-[12px] text-text-muted">
										<Icon src={FiList} size="13" />
										Status
									</span>
									<select
										bind:value={panelStatus}
										class="rounded-lg border border-border bg-bg-tertiary px-2 py-1 text-[12px] text-text-primary outline-none focus:border-accent"
									>
										<option value="todo">To Do</option>
										<option value="doing">In Progress</option>
										<option value="done">Done</option>
									</select>
								</div>

								<div class="flex items-center justify-between">
									<span class="flex items-center gap-2 text-[12px] text-text-muted">
										<span class="text-[14px] font-bold">!</span>
										Priority
									</span>
									<select
										bind:value={panelPriority}
										class="rounded-lg border border-border bg-bg-tertiary px-2 py-1 text-[12px] text-text-primary outline-none focus:border-accent"
									>
										<option value="">None</option>
										<option value="low">Low</option>
										<option value="medium">Medium</option>
										<option value="high">High</option>
									</select>
								</div>

								<div class="flex items-center justify-between">
									<span class="flex items-center gap-2 text-[12px] text-text-muted">
										<Icon src={FiCalendar} size="13" />
										Due Date
									</span>
									<input
										type="date"
										bind:value={panelDue}
										class="rounded-lg border border-border bg-bg-tertiary px-2 py-1 text-[12px] text-text-primary outline-none focus:border-accent"
									/>
								</div>

								<div class="flex items-center justify-between">
									<span class="flex items-center gap-2 text-[12px] text-text-muted">
										<Icon src={FiTag} size="13" />
										Tags
									</span>
									<input
										type="text"
										bind:value={panelTags}
										placeholder="work, personal"
										class="w-36 rounded-lg border border-border bg-bg-tertiary px-2 py-1 text-[12px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
									/>
								</div>
							</div>
						</div>

						<div class="border-t border-border px-4 py-3">
							<button
								type="submit"
								disabled={!panelTitle.trim()}
								class="w-full rounded-lg bg-accent py-2 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40"
							>
								{panelMode === "add" ? "Add Task" : "Save Changes"}
							</button>
						</div>
					</form>
				</div>
			{/if}
		</div>
	</div>

	<!-- Mobile FAB -->
	{#if !panelOpen}
		<button
			onclick={openAdd}
			class="fixed bottom-6 right-5 z-30 flex h-14 w-14 items-center justify-center rounded-full bg-accent text-white shadow-lg transition-transform active:scale-95 md:hidden"
			style="bottom: max(env(safe-area-inset-bottom, 0px) + 16px, 24px)"
		>
			<Icon src={FiPlus} size="24" />
		</button>
	{/if}
</div>

<!-- Mobile sidebar drawer -->
{#if showSidebar}
	<button
		class="fixed inset-0 z-40 bg-black/50 backdrop-blur-sm md:hidden"
		onclick={() => (showSidebar = false)}
		tabindex="-1"
		aria-label="Close sidebar"
	></button>
	<div class="fixed inset-y-0 left-0 z-50 flex w-64 flex-col bg-bg border-r border-border md:hidden"
		style="padding-top: max(env(safe-area-inset-top, 0px), 12px)"
	>
		<div class="px-4 py-3 border-b border-border">
			<span class="text-[14px] font-semibold text-text-primary">Tasks</span>
		</div>

		<div class="flex flex-1 flex-col overflow-y-auto px-2 py-2">
			<span class="px-2 pb-1 pt-1 text-[10px] font-semibold uppercase tracking-widest text-text-muted">Smart Lists</span>
			{#each smartLists as item}
				<button
					onclick={() => { taskStore.smartList = item.key; taskStore.search = ""; showSearch = false; showSidebar = false; }}
					class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-[14px] transition-colors
						{taskStore.smartList === item.key && !taskStore.search ? 'bg-accent/10 text-accent font-medium' : 'text-text-secondary hover:bg-overlay-light'}"
				>
					<Icon
						src={item.icon}
						size="17"
						className={taskStore.smartList === item.key && !taskStore.search ? 'text-accent' : 'text-text-muted'}
					/>
					<span class="flex-1 text-left">{item.label}</span>
					<span class="text-[12px] tabular-nums text-text-muted">{taskStore.counts[item.key]}</span>
				</button>
			{/each}

			{#if taskStore.allTags.length > 0}
				<span class="px-2 pb-1 pt-4 text-[10px] font-semibold uppercase tracking-widest text-text-muted">Tags</span>
				{#each taskStore.allTags as tag}
					<button
						onclick={() => { taskStore.filterTag = taskStore.filterTag === tag ? "" : tag; showSidebar = false; }}
						class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-[14px] transition-colors
							{taskStore.filterTag === tag ? 'bg-accent/10 text-accent' : 'text-text-secondary hover:bg-overlay-light'}"
					>
						<Icon src={FiTag} size="14" className={taskStore.filterTag === tag ? 'text-accent' : 'text-text-muted'} />
						<span>{tag}</span>
					</button>
				{/each}
			{/if}
		</div>
	</div>
{/if}

<!-- Mobile detail panel -->
{#if panelOpen}
	<div class="fixed inset-0 z-40 flex flex-col bg-bg md:hidden">
		<form onsubmit={handleSubmit} class="flex flex-1 flex-col overflow-hidden">
			<div
				class="flex items-center gap-2 border-b border-border px-3 py-2"
				style="padding-top: max(env(safe-area-inset-top, 0px), 10px)"
			>
				<button
					type="button"
					onclick={closePanel}
					class="flex h-9 w-9 items-center justify-center rounded-lg text-text-secondary hover:bg-overlay-light"
				>
					<Icon src={FiArrowLeft} size="18" />
				</button>
				<span class="flex-1"></span>
				{#if panelMode === "edit"}
					<button
						type="button"
						onclick={handleDelete}
						class="flex h-9 w-9 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-danger"
					>
						<Icon src={FiTrash2} size="17" />
					</button>
				{/if}
				<button
					type="submit"
					disabled={!panelTitle.trim()}
					class="flex h-9 items-center rounded-lg bg-accent px-4 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40"
				>
					{panelMode === "add" ? "Add" : "Save"}
				</button>
			</div>

			<div class="flex flex-1 flex-col gap-4 overflow-y-auto p-4">
				<!-- svelte-ignore a11y_autofocus -->
				<input
					type="text"
					bind:value={panelTitle}
					placeholder="Task title"
					class="bg-transparent text-[17px] font-semibold text-text-primary placeholder:text-text-muted outline-none"
					autofocus={panelMode === "add"}
				/>

				<textarea
					bind:value={panelDescription}
					placeholder="Add notes..."
					rows="4"
					class="resize-none bg-transparent text-[14px] text-text-secondary placeholder:text-text-muted outline-none"
				></textarea>

				<div class="flex flex-col gap-0 border-t border-border pt-2">
					<div class="flex items-center justify-between py-3 border-b border-border/50">
						<span class="flex items-center gap-3 text-[14px] text-text-secondary">
							<Icon src={FiList} size="16" className="text-text-muted" />
							Status
						</span>
						<select
							bind:value={panelStatus}
							class="rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[13px] text-text-primary outline-none"
						>
							<option value="todo">To Do</option>
							<option value="doing">In Progress</option>
							<option value="done">Done</option>
						</select>
					</div>

					<div class="flex items-center justify-between py-3 border-b border-border/50">
						<span class="flex items-center gap-3 text-[14px] text-text-secondary">
							<span class="flex h-4 w-4 items-center justify-center text-[15px] font-bold text-text-muted">!</span>
							Priority
						</span>
						<select
							bind:value={panelPriority}
							class="rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[13px] text-text-primary outline-none"
						>
							<option value="">None</option>
							<option value="low">Low</option>
							<option value="medium">Medium</option>
							<option value="high">High</option>
						</select>
					</div>

					<div class="flex items-center justify-between py-3 border-b border-border/50">
						<span class="flex items-center gap-3 text-[14px] text-text-secondary">
							<Icon src={FiCalendar} size="16" className="text-text-muted" />
							Due Date
						</span>
						<input
							type="date"
							bind:value={panelDue}
							class="rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[13px] text-text-primary outline-none"
						/>
					</div>

					<div class="flex items-center justify-between py-3">
						<span class="flex items-center gap-3 text-[14px] text-text-secondary">
							<Icon src={FiTag} size="16" className="text-text-muted" />
							Tags
						</span>
						<input
							type="text"
							bind:value={panelTags}
							placeholder="work, personal"
							class="w-40 rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[13px] text-text-primary placeholder:text-text-muted outline-none"
						/>
					</div>
				</div>
			</div>
		</form>
	</div>
{/if}

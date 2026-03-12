<script lang="ts">
import { onMount } from "svelte";
import { taskStore, type SmartList } from "$lib/stores/task.svelte";
import type { Task } from "$lib/api";
import { Select } from "bits-ui";
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
  FiChevronDown,
  FiCornerDownRight,
  FiAlertCircle,
  FiRepeat,
  FiClock,
  FiBell,
} from "svelte-icons-pack/fi";
import TaskContextMenu from "$lib/components/TaskContextMenu.svelte";
import DatePicker from "$lib/components/DatePicker.svelte";
import RecurrencePicker from "$lib/components/RecurrencePicker.svelte";
import BlockPicker from "$lib/components/BlockPicker.svelte";
import { blockStore } from "$lib/stores/block.svelte";
import { tagColorStore } from "$lib/stores/tagColor.svelte";
import { formatTime24 } from "$lib/utils";
import type { Block } from "$lib/api";

let panelOpen = $state(false);
let panelMode = $state<"add" | "edit">("add");
let panelTitle = $state("");
let panelDescription = $state("");
let panelPriority = $state("");
let panelDue = $state("");
let panelTags = $state("");
let panelStatus = $state("todo");
let panelRecurrence = $state("");
let panelReminders = $state<string[]>([]);
let panelParentId = $state("");

let expandedParents = $state(new Set<string>());

let quickAddValue = $state("");
let quickAddFocused = $state(false);

let showSidebar = $state(false);
let showSearch = $state(false);
let colorPickerTag = $state<string | null>(null);
let colorPickerPos = $state<{ x: number; y: number }>({ x: 0, y: 0 });

function openColorPicker(e: MouseEvent, tag: string) {
  e.stopPropagation();
  if (colorPickerTag === tag) {
    colorPickerTag = null;
    return;
  }
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
  colorPickerPos = { x: rect.right + 4, y: rect.top };
  colorPickerTag = tag;
}

interface TagNode {
  label: string;
  path: string; // unique key, e.g. "work::frontend"
  fullTag: string | null; // null = group only (no exact tag match)
  children: TagNode[];
}

let tagTree = $derived.by(() => {
  const root: TagNode[] = [];
  for (const tag of taskStore.allTags) {
    const parts = tag.split("::");
    let level = root;
    let path = "";
    for (let i = 0; i < parts.length; i++) {
      path = path ? `${path}::${parts[i]}` : parts[i];
      let node = level.find((n) => n.label === parts[i]);
      if (!node) {
        node = {
          label: parts[i],
          path,
          fullTag: null,
          children: [],
        };
        level.push(node);
      }
      if (i === parts.length - 1) node.fullTag = tag;
      level = node.children;
    }
  }
  return root;
});

function isTagActive(node: TagNode): boolean {
  if (node.fullTag && taskStore.filterTags.includes(node.fullTag)) return true;
  return node.children.some((c) => isTagActive(c));
}

let expandedTagGroups = $state(new Set<string>());

function toggleTagGroup(path: string) {
  const next = new Set(expandedTagGroups);
  if (next.has(path)) next.delete(path);
  else next.add(path);
  expandedTagGroups = next;
}

function closeColorPicker(e: MouseEvent) {
  if (
    colorPickerTag &&
    !(e.target as HTMLElement).closest("[data-color-picker]")
  ) {
    colorPickerTag = null;
  }
}

let dragOverCol = $state<string | null>(null);
let draggingId = $state<string | null>(null);
let dragGhost: HTMLElement | null = null;
let dragOverTaskId = $state<string | null>(null);
let dragOverBefore = $state(true);

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
  // Auto-expand parents with visible subtasks
  const next = new Set(expandedParents);
  for (const t of taskStore.filtered) {
    if (t.parentId && !next.has(t.parentId)) {
      next.add(t.parentId);
    }
  }
  expandedParents = next;

  const selectParam = new URLSearchParams(window.location.search).get("select");
  const preselect = selectParam || taskStore.selectedId;
  if (preselect) {
    const task = taskStore.tasks.find((t) => t.id === preselect);
    if (task) openDetail(task);
    if (selectParam) {
      const url = new URL(window.location.href);
      url.searchParams.delete("select");
      history.replaceState({}, "", url.toString());
    }
  }
});

$effect(() => {
  // Track dependencies
  taskStore.smartList;
  taskStore.filterTags;
  taskStore.search;
  // Auto-expand parents with visible subtasks
  const next = new Set<string>();
  for (const t of taskStore.filtered) {
    if (t.parentId) next.add(t.parentId);
  }
  expandedParents = next;
});

const smartLists: { key: SmartList; label: string; icon: typeof FiSun }[] = [
  { key: "all", label: "All", icon: FiList },
  { key: "today", label: "Today", icon: FiSun },
  { key: "tomorrow", label: "Tomorrow", icon: FiCalendar },
  { key: "next7", label: "Next 7 Days", icon: FiCalendar },
  { key: "overdue", label: "Overdue", icon: FiAlertCircle },
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

function openAdd(parentId = "") {
  panelMode = "add";
  taskStore.selectedId = "";
  panelTitle = "";
  panelDescription = "";
  panelPriority = "";
  panelDue = "";
  panelRecurrence = "";
  panelReminders = [];
  panelTags = "";
  panelStatus = "todo";
  panelParentId = parentId;
  panelOpen = true;
}

function openDetail(task: Task) {
  panelMode = "edit";
  taskStore.selectedId = task.id;
  panelTitle = task.title;
  panelDescription = task.description ?? "";
  panelPriority = task.priority ?? "";
  panelDue = task.due ?? "";
  panelRecurrence = task.recurrence ?? "";
  panelReminders = task.reminders ?? [];
  panelTags = task.tags?.join(", ") ?? "";
  panelStatus = task.status;
  panelParentId = task.parentId ?? "";
  panelOpen = true;
}

function toggleExpand(id: string) {
  const next = new Set(expandedParents);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  expandedParents = next;
}

function closePanel() {
  panelOpen = false;
}

function navigateTask(direction: -1 | 1) {
  // Build visible list: top-level + expanded subtasks
  const list: Task[] = [];
  for (const t of taskStore.topLevelFiltered) {
    list.push(t);
    if (expandedParents.has(t.id)) {
      for (const sub of taskStore.subtasksOf(t.id)) {
        list.push(sub);
      }
    }
  }
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

// Auto-save: update the task immediately when a field changes
async function autoSave(patch: Partial<Task>) {
  if (panelMode !== "edit" || !taskStore.selectedId) return;
  await taskStore.update(taskStore.selectedId, patch);
}

// Debounced auto-save for text fields
let saveTimer: ReturnType<typeof setTimeout> | null = null;
function debouncedAutoSave(patch: Partial<Task>) {
  if (saveTimer) clearTimeout(saveTimer);
  saveTimer = setTimeout(() => autoSave(patch), 400);
}

let tagSuggestionOpen = $state(false);
let tagInputEl = $state<HTMLInputElement | null>(null);

let tagSuggestions = $derived.by(() => {
  if (!tagSuggestionOpen || !panelTags) return [];
  // Get the current tag being typed (after the last comma)
  const parts = panelTags.split(",");
  const current = parts[parts.length - 1].trim().toLowerCase();
  if (!current) return [];
  const existing = parts.slice(0, -1).map((t) => t.trim().toLowerCase());
  return taskStore.allTags
    .filter(
      (t) =>
        t.toLowerCase().includes(current) &&
        !existing.includes(t.toLowerCase()),
    )
    .slice(0, 8);
});

function acceptTagSuggestion(tag: string) {
  const parts = panelTags
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean);
  parts[parts.length - 1] = tag;
  panelTags = parts.join(", ") + ", ";
  tagSuggestionOpen = false;
  tagInputEl?.focus();
}

async function handleAddSubmit(e: SubmitEvent) {
  e.preventDefault();
  if (!panelTitle.trim()) return;

  const data: Partial<Task> = {
    title: panelTitle.trim(),
    description: panelDescription.trim() || undefined,
    priority: panelPriority || undefined,
    due: panelDue || undefined,
    recurrence: panelRecurrence || undefined,
    reminders: panelReminders.length > 0 ? panelReminders : undefined,
    tags: parseTags(panelTags),
    status: panelStatus,
    parentId: panelParentId || undefined,
  };

  await taskStore.add(data);
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
    tags:
      taskStore.filterTags.length > 0 ? [...taskStore.filterTags] : undefined,
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
  if (p === "high") return "bg-error";
  if (p === "medium") return "bg-warning";
  if (p === "low") return "bg-text-muted";
  return "";
}

function isOverdue(due?: string): boolean {
  if (!due) return false;
  const datePart = due.includes("T") ? due.split("T")[0] : due;
  return datePart < new Date().toISOString().slice(0, 10);
}

function formatDate(due: string): string {
  const datePart = due.includes("T") ? due.split("T")[0] : due;
  const timePart = due.includes("T") ? due.split("T")[1] : "";
  const today = new Date().toISOString().slice(0, 10);
  const tomorrow = new Date(Date.now() + 86400000).toISOString().slice(0, 10);

  let label: string;
  if (datePart === today) label = "Today";
  else if (datePart === tomorrow) label = "Tomorrow";
  else {
    const d = new Date(datePart + "T00:00:00");
    label = d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
  }

  if (timePart) {
    const [h, m] = timePart.split(":");
    const hour = parseInt(h);
    const suffix = hour >= 12 ? "pm" : "am";
    const h12 = hour === 0 ? 12 : hour > 12 ? hour - 12 : hour;
    label += ` ${h12}:${m}${suffix}`;
  }

  return label;
}

async function handleDrop(e: DragEvent, status: string) {
  e.preventDefault();
  const id = e.dataTransfer?.getData("text/plain");
  const overTaskId = dragOverTaskId;
  const before = dragOverBefore;
  dragOverCol = null;
  dragOverTaskId = null;

  if (!id) return;

  const draggingTask = taskStore.tasks.find((t) => t.id === id);
  if (!draggingTask) return;

  const needsStatusChange = draggingTask.status !== status;
  const col =
    taskStore.kanbanColumns[status as keyof typeof taskStore.kanbanColumns];

  if (overTaskId && overTaskId !== id) {
    const colWithoutDragging = col.filter((t) => t.id !== id);
    const targetIdx = colWithoutDragging.findIndex((t) => t.id === overTaskId);
    if (targetIdx !== -1) {
      let newOrder: number;
      if (before) {
        const prev = targetIdx > 0 ? colWithoutDragging[targetIdx - 1] : null;
        const tgt = colWithoutDragging[targetIdx];
        newOrder = prev ? (prev.order + tgt.order) / 2 : tgt.order - 1;
      } else {
        const next =
          targetIdx < colWithoutDragging.length - 1
            ? colWithoutDragging[targetIdx + 1]
            : null;
        const tgt = colWithoutDragging[targetIdx];
        newOrder = next ? (tgt.order + next.order) / 2 : tgt.order + 1;
      }
      if (needsStatusChange) await taskStore.moveStatus(id, status);
      await taskStore.reorder(id, newOrder);
      return;
    }
  }

  if (needsStatusChange) await taskStore.moveStatus(id, status);
}

function handleTaskDragOver(e: DragEvent, taskId: string) {
  e.preventDefault();
  if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
  if (draggingId === taskId) return;
  dragOverTaskId = taskId;
  const el = e.currentTarget as HTMLElement;
  const rect = el.getBoundingClientRect();
  dragOverBefore = e.clientY < rect.top + rect.height / 2;
}

function handleTaskDragLeave(e: DragEvent, taskId: string) {
  const target = e.currentTarget as HTMLElement;
  const related = e.relatedTarget as Node | null;
  if (related && target.contains(related)) return;
  if (dragOverTaskId === taskId) dragOverTaskId = null;
}

async function handleListDrop(e: DragEvent, targetId: string, list: Task[]) {
  e.preventDefault();
  e.stopPropagation();
  const id = e.dataTransfer?.getData("text/plain");
  const before = dragOverBefore;
  dragOverTaskId = null;
  draggingId = null;
  if (dragGhost) {
    dragGhost.remove();
    dragGhost = null;
  }

  if (!id || id === targetId) return;

  const listWithoutDragging = list.filter((t) => t.id !== id);
  const targetIdx = listWithoutDragging.findIndex((t) => t.id === targetId);
  if (targetIdx === -1) return;

  let newOrder: number;
  if (before) {
    const prev = targetIdx > 0 ? listWithoutDragging[targetIdx - 1] : null;
    const tgt = listWithoutDragging[targetIdx];
    newOrder = prev ? (prev.order + tgt.order) / 2 : tgt.order - 1;
  } else {
    const next =
      targetIdx < listWithoutDragging.length - 1
        ? listWithoutDragging[targetIdx + 1]
        : null;
    const tgt = listWithoutDragging[targetIdx];
    newOrder = next ? (tgt.order + next.order) / 2 : tgt.order + 1;
  }

  await taskStore.reorder(id, newOrder);
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
  ghost.style.transform = "scale(1.02)";
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

let kanbanCols = $derived(
  taskStore.smartList === "done"
    ? [
        { key: "todo", label: "To Do", color: "text-text-secondary" },
        { key: "doing", label: "In Progress", color: "text-accent" },
        { key: "done", label: "Done", color: "text-success" },
      ]
    : [
        { key: "todo", label: "To Do", color: "text-text-secondary" },
        { key: "doing", label: "In Progress", color: "text-accent" },
      ],
);
</script>

{#snippet tagTreeNodes(nodes: TagNode[], depth: number)}
  {#each nodes as node}
    {@const active = isTagActive(node)}
    {@const tc = node.fullTag ? tagColorStore.get(node.fullTag) : null}
    {@const hasChildren = node.children.length > 0}
    {@const expanded = expandedTagGroups.has(node.path)}
    <div class="group flex items-center rounded-lg transition-colors
      {active ? 'bg-accent/10' : 'hover:bg-overlay-light'}">
      {#if hasChildren && node.fullTag}
        <button
          onclick={() => toggleTagGroup(node.path)}
          class="flex shrink-0 items-center justify-center w-5 h-5 rounded transition-transform {expanded ? '' : '-rotate-90'}"
          style="margin-left:{4 + depth * 14}px"
        >
          <Icon src={FiChevronRight} size="13" className="{active ? 'text-accent' : 'text-text-muted'}" />
        </button>
        <button
          onclick={(e) => taskStore.toggleTag(node.fullTag!, e.metaKey || e.ctrlKey)}
          class="flex flex-1 items-center gap-2.5 py-1.5 text-[13px] transition-colors
            {active ? 'text-accent' : 'text-text-secondary hover:text-text-primary'}"
        >
          {#if tc}
            <span class="h-2.5 w-2.5 shrink-0 rounded-full" style="background:{tc}"></span>
          {/if}
          <span>{node.label}</span>
        </button>
      {:else}
        <button
          onclick={(e) => {
            if (hasChildren) toggleTagGroup(node.path);
            else if (node.fullTag) taskStore.toggleTag(node.fullTag, e.metaKey || e.ctrlKey);
          }}
          class="flex flex-1 items-center gap-2.5 px-2.5 py-1.5 text-[13px] transition-colors
            {active ? 'text-accent' : 'text-text-secondary hover:text-text-primary'}"
          style="padding-left:{10 + depth * 14}px"
        >
          {#if tc}
            <span class="h-2.5 w-2.5 shrink-0 rounded-full" style="background:{tc}"></span>
          {:else if hasChildren}
            <Icon src={FiChevronRight} size="13" className="shrink-0 transition-transform {expanded ? 'rotate-90' : ''} {active ? 'text-accent' : 'text-text-muted'}" />
          {:else}
            <Icon src={FiTag} size="13" className="shrink-0 {active ? 'text-accent' : 'text-text-muted'}" />
          {/if}
          <span>{node.label}</span>
        </button>
      {/if}
      {#if node.fullTag}
        <button
          onclick={(e) => openColorPicker(e, node.fullTag!)}
          class="mr-1 flex h-5 w-5 items-center justify-center rounded opacity-0 transition-opacity group-hover:opacity-100 hover:bg-overlay-light"
          title="Set color"
        >
          <span class="h-2 w-2 rounded-full {tc ? '' : 'border border-text-muted/40'}" style={tc ? `background:${tc}` : ""}></span>
        </button>
      {/if}
    </div>
    {#if hasChildren && expanded}
      {@render tagTreeNodes(node.children, depth + 1)}
    {/if}
  {/each}
{/snippet}

{#snippet selectIndicator(selected: boolean)}
  <span
    class="flex h-3.5 w-3.5 items-center justify-center rounded-full border border-border-light {selected ? 'bg-accent border-accent' : ''}"
  >
    {#if selected}
      <Icon src={FiCheck} size="8" className="text-white" />
    {/if}
  </span>
{/snippet}

{#snippet detailPanel(mobile: boolean)}
  {@const sz = mobile ? "text-[14px]" : "text-[12px]"}
  {@const szLabel = mobile ? "text-[14px]" : "text-[12px]"}
  {@const gap = mobile ? "gap-3" : "gap-2"}
  {@const iconSz = mobile ? "16" : "13"}
  {@const py = mobile ? "py-3" : "py-2.5"}

  <div class="flex flex-1 flex-col gap-4 overflow-y-auto p-4">
    <input
      type="text"
      bind:value={panelTitle}
      placeholder="Task title"
      class="bg-transparent {mobile ? 'text-[17px] font-semibold' : 'text-[15px] font-medium'} text-text-primary placeholder:text-text-muted outline-none"
      onblur={() => {
        if (panelTitle.trim()) debouncedAutoSave({ title: panelTitle.trim() });
      }}
    />

    <textarea
      bind:value={panelDescription}
      placeholder="Add notes..."
      rows={mobile ? 4 : 3}
      class="resize-none bg-transparent {mobile ? 'text-[14px]' : 'text-[13px]'} text-text-secondary placeholder:text-text-muted outline-none"
      onblur={() => debouncedAutoSave({ description: panelDescription.trim() || undefined })}
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
          value={panelStatus}
          onValueChange={(v) => {
            panelStatus = v;
            if (v === "done") {
              taskStore.complete(taskStore.selectedId);
            } else {
              autoSave({ status: v });
            }
          }}
        >
          <Select.Trigger
            class="flex items-center gap-1.5 rounded-lg border border-border bg-bg-tertiary px-2 py-1 {sz} text-text-primary outline-none hover:border-border-light focus:border-accent"
          >
            {statusOptions.find((s) => s.value === panelStatus)?.label ?? "To Do"}
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
          value={panelPriority}
          onValueChange={(v) => {
            panelPriority = v;
            autoSave({ priority: v || undefined } as Partial<Task>);
          }}
        >
          <Select.Trigger
            class="flex items-center gap-1.5 rounded-lg border border-border bg-bg-tertiary px-2 py-1 {sz} text-text-primary outline-none hover:border-border-light focus:border-accent"
          >
            <span class="flex items-center gap-1.5">
              {#if panelPriority}
                <span class="h-2 w-2 rounded-full {priorityColor(panelPriority)}"></span>
              {/if}
              {priorityOptions.find((p) => p.value === panelPriority)?.label ?? "None"}
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
          value={panelDue}
          onchange={(v) => {
            panelDue = v;
            autoSave({ due: v || undefined } as Partial<Task>);
          }}
        />
      </div>

      <!-- Reminders -->
      {#if panelDue}
      <div class="flex items-center justify-between {py} border-b border-border/50">
        <span class="flex items-center {gap} {szLabel} text-text-secondary">
          <Icon src={FiBell} size={iconSz} className="text-text-muted" />
          Reminders
        </span>
        <div class="flex flex-wrap justify-end gap-1">
          {#each [{ key: "0", label: "At due" }, { key: "15m", label: "15m" }, { key: "30m", label: "30m" }, { key: "1h", label: "1h" }, { key: "2h", label: "2h" }, { key: "1d", label: "1d" }, { key: "2d", label: "2d" }, { key: "1w", label: "1w" }] as opt}
            {@const active = panelReminders.includes(opt.key)}
            {@const isTimeBased = !["0", "1d", "2d", "1w"].includes(opt.key)}
            {@const hasTime = panelDue.includes("T")}
            {#if !isTimeBased || hasTime}
              <button
                type="button"
                class="rounded-md border px-1.5 py-0.5 text-[10px] font-medium transition-colors
                  {active
                    ? 'border-accent bg-accent/15 text-accent'
                    : 'border-border bg-bg-tertiary text-text-muted hover:border-border-light hover:text-text-secondary'}"
                onclick={() => {
                  if (active) {
                    panelReminders = panelReminders.filter((r) => r !== opt.key);
                  } else {
                    panelReminders = [...panelReminders, opt.key];
                  }
                  autoSave({ reminders: panelReminders } as Partial<Task>);
                }}
              >
                {opt.label}
              </button>
            {/if}
          {/each}
        </div>
      </div>
      {/if}

      <!-- Recurrence -->
      <div class="flex items-center justify-between {py} border-b border-border/50">
        <span class="flex items-center {gap} {szLabel} text-text-secondary">
          <Icon src={FiRepeat} size={iconSz} className="text-text-muted" />
          Repeat
        </span>
        <RecurrencePicker
          value={panelRecurrence}
          onchange={(v) => {
            panelRecurrence = v;
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
        <div class="relative {mobile ? 'w-40' : 'w-36'}">
          <input
            bind:this={tagInputEl}
            type="text"
            bind:value={panelTags}
            placeholder="work, personal"
            class="w-full rounded-lg border border-border bg-bg-tertiary px-2 py-1 {sz} text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
            onfocus={() => (tagSuggestionOpen = true)}
            oninput={() => (tagSuggestionOpen = true)}
            onblur={() => {
              setTimeout(() => (tagSuggestionOpen = false), 150);
              const tags = parseTags(panelTags);
              autoSave({ tags: tags ?? [] } as Partial<Task>);
            }}
          />
          {#if tagSuggestions.length > 0}
            <div class="absolute left-0 right-0 top-full z-50 mt-1 max-h-32 overflow-y-auto rounded-lg border border-border bg-bg-secondary shadow-elevated">
              {#each tagSuggestions as suggestion}
                <button
                  type="button"
                  class="w-full px-2 py-1.5 text-left text-[12px] text-text-primary hover:bg-overlay-light"
                  onmousedown={(e) => {
                    e.preventDefault();
                    acceptTagSuggestion(suggestion);
                  }}
                >
                  {suggestion}
                </button>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <!-- Parent -->
      <div class="flex items-center justify-between {py}">
        <span class="flex items-center {gap} {szLabel} text-text-secondary">
          <Icon src={FiCornerDownRight} size={iconSz} className="text-text-muted" />
          Parent
        </span>
        <Select.Root
          type="single"
          value={panelParentId}
          onValueChange={(v) => {
            panelParentId = v;
            autoSave({ parentId: v || undefined } as Partial<Task>);
          }}
        >
          <Select.Trigger
            class="{mobile ? 'w-40' : 'w-36'} flex items-center gap-1.5 truncate rounded-lg border border-border bg-bg-tertiary px-2 py-1 {sz} text-text-primary outline-none hover:border-border-light focus:border-accent"
          >
            <span class="flex-1 truncate text-left">
              {#if panelParentId}
                {taskStore.tasks.find((t) => t.id === panelParentId)?.title ?? "None"}
              {:else}
                None
              {/if}
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
                {#each taskStore.tasks.filter((t) => t.id !== taskStore.selectedId && !t.parentId) as t}
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
    {#if panelMode === "edit" && taskStore.selectedId}
      {@const taskBlocks = blockStore.forTask(taskStore.selectedId)}
      <div class="flex flex-col gap-1 border-t border-border pt-3">
        <div class="flex items-center justify-between">
          <span class="text-[11px] font-semibold uppercase tracking-widest text-text-muted">Time Blocks</span>
          <BlockPicker oncreate={(startMs, endMs) => blockStore.add({ taskId: taskStore.selectedId!, startAtMs: startMs, endAtMs: endMs })} />
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

    <!-- Subtasks section (edit mode only) -->
    {#if panelMode === "edit" && taskStore.selectedId}
      {@const panelSubtasks = taskStore.subtasksOf(taskStore.selectedId)}
      {#if panelSubtasks.length > 0}
        <div class="flex flex-col gap-1 border-t border-border pt-3">
          <span class="text-[11px] font-semibold uppercase tracking-widest text-text-muted">Subtasks</span>
          {#each panelSubtasks as sub}
            <button
              onclick={() => openDetail(sub)}
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
        onclick={() => openAdd(taskStore.selectedId)}
        class="flex items-center {mobile ? 'gap-2 text-[14px]' : 'gap-1.5 text-[12px]'} text-text-muted hover:text-accent transition-colors"
      >
        <Icon src={FiPlus} size={mobile ? "14" : "12"} />
        Add subtask
      </button>
    {/if}
  </div>
{/snippet}

<svelte:window onkeydown={handleKeydown} />

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="flex h-full" onclick={closeColorPicker}>
  <!-- Desktop sidebar -->
  <div class="hidden w-52 shrink-0 flex-col border-r border-border bg-bg md:flex">
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

    {#if taskStore.allTags.length > 0}
      <div class="flex flex-col px-1.5 py-2 border-t border-border">
        <span class="px-2 pb-1 text-[10px] font-semibold uppercase tracking-widest text-text-muted">Tags</span>
        {@render tagTreeNodes(tagTree, 0)}
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
      {#each taskStore.filterTags as tag}
        {@const tc = tagColorStore.get(tag)}
        {@const tagLabel = tag.includes("::") ? tag.split("::").pop() : tag}
        <span class="flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] {tc ? '' : 'bg-accent/10 text-accent'}"
          style={tc ? `background:${tc}18;color:${tc}` : ""}
        >
          {#if tc}
            <span class="h-1.5 w-1.5 rounded-full" style="background:{tc}"></span>
          {:else}
            <Icon src={FiTag} size="10" />
          {/if}
          {tagLabel}
          <button onclick={() => taskStore.toggleTag(tag)} class="ml-0.5 hover:text-text-primary">
            <Icon src={FiX} size="10" />
          </button>
        </span>
      {/each}
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
          onclick={() => openAdd()}
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
              {#each taskStore.topLevelFiltered as task (task.id)}
                {@const subtasks = taskStore.subtasksOf(task.id)}
                {@const hasChildren = subtasks.length > 0}
                {@const isExpanded = expandedParents.has(task.id)}
                <div role="listitem"
                  class="relative overflow-hidden {draggingId === task.id ? 'opacity-40' : ''}"
                  draggable="true"
                  ondragstart={(e) => handleDragStart(e, task.id)}
                  ondragend={handleDragEnd}
                  ondragover={(e) => handleTaskDragOver(e, task.id)}
                  ondragleave={(e) => handleTaskDragLeave(e, task.id)}
                  ondrop={(e) => handleListDrop(e, task.id, taskStore.topLevelFiltered)}
                >
                  {#if dragOverTaskId === task.id && draggingId !== task.id}
                    <div class="pointer-events-none absolute inset-x-0 z-10 h-0.5 bg-accent {dragOverBefore ? 'top-0' : 'bottom-0'}"></div>
                  {/if}
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

                  <TaskContextMenu {task} onOpenDetail={openDetail} onAddSubtask={openAdd}>
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
                        <span class="flex items-center gap-1.5">
                          {#if task.priority && task.status !== "done"}
                            <span class="h-2 w-2 shrink-0 rounded-full {priorityColor(task.priority)}" title="{task.priority} priority"></span>
                          {/if}
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
                              {@const tc = tagColorStore.get(tag)}
                              {@const tagLabel = tag.includes("::") ? tag.split("::").pop() : tag}
                              <span class="flex items-center gap-0.5 text-[11px] text-text-muted">
                                {#if tc}
                                  <span class="h-1.5 w-1.5 rounded-full" style="background:{tc}"></span>
                                {:else}
                                  <Icon src={FiTag} size="9" />
                                {/if}
                                {tagLabel}
                              </span>
                            {/each}
                          {/if}
                        </span>
                      </span>

                      {#if hasChildren}
                        <span
                          role="button"
                          tabindex="-1"
                          onclick={(e) => { e.stopPropagation(); toggleExpand(task.id); }}
                          onkeydown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); e.stopPropagation(); toggleExpand(task.id); } }}
                          class="flex h-5 w-5 shrink-0 items-center justify-center rounded text-text-muted hover:text-text-secondary transition-transform {isExpanded ? '' : '-rotate-90'}"
                        >
                          <span class="flex items-center gap-1">
                            <span class="text-[10px] text-text-muted tabular-nums">
                              {subtasks.filter((s) => s.status === "done").length}/{subtasks.length}
                            </span>
                            <Icon src={FiChevronDown} size="12" />
                          </span>
                        </span>
                      {/if}
                    </button>
                  </TaskContextMenu>
                  <div class="mx-4 border-b border-border/50 md:mx-5"></div>
                </div>

                <!-- Subtasks -->
                {#if hasChildren && isExpanded}
                  {@const sortedSubtasks = [...subtasks].sort((a, b) => (a.order || 0) - (b.order || 0))}
                  {#each sortedSubtasks as sub (sub.id)}
                    <div role="listitem"
                      class="relative overflow-hidden {draggingId === sub.id ? 'opacity-40' : ''}"
                      draggable="true"
                      ondragstart={(e) => handleDragStart(e, sub.id)}
                      ondragend={handleDragEnd}
                      ondragover={(e) => handleTaskDragOver(e, sub.id)}
                      ondragleave={(e) => handleTaskDragLeave(e, sub.id)}
                      ondrop={(e) => handleListDrop(e, sub.id, sortedSubtasks)}
                    >
                      {#if dragOverTaskId === sub.id && draggingId !== sub.id}
                        <div class="pointer-events-none absolute inset-x-0 z-10 h-0.5 bg-accent {dragOverBefore ? 'top-0' : 'bottom-0'}"></div>
                      {/if}
                      {#if swipeTaskId === sub.id && swipeX > 0}
                        <div class="absolute inset-0 flex items-center bg-success/15 px-4 text-success">
                          <Icon src={FiCheck} size="16" />
                          <span class="ml-2 text-[12px] font-medium">Done</span>
                        </div>
                      {/if}
                      {#if swipeTaskId === sub.id && swipeX < 0}
                        <div class="absolute inset-0 flex items-center justify-end bg-danger/15 px-4 text-danger">
                          <span class="mr-2 text-[12px] font-medium">Delete</span>
                          <Icon src={FiTrash2} size="16" />
                        </div>
                      {/if}

                      <TaskContextMenu task={sub} onOpenDetail={openDetail}>
                        <button
                          class="relative flex w-full items-center gap-3 bg-bg pl-10 pr-4 py-2 text-left transition-colors duration-75 md:pl-12 md:pr-5
                            {panelOpen && taskStore.selectedId === sub.id ? 'bg-accent/5' : 'hover:bg-overlay-subtle'}"
                          style={swipeTaskId === sub.id && swipeX !== 0 ? `transform: translateX(${swipeX}px)` : ""}
                          onclick={() => openDetail(sub)}
                          ontouchstart={(e) => handleTouchStart(e, sub.id)}
                          ontouchmove={handleTouchMove}
                          ontouchend={handleTouchEnd}
                        >
                          <Icon src={FiCornerDownRight} size="11" className="shrink-0 text-text-muted/40" />
                          <span
                            role="checkbox"
                            tabindex="-1"
                            aria-checked={sub.status === "done"}
                            onclick={(e) => {
                              e.stopPropagation();
                              sub.status === "done"
                                ? taskStore.update(sub.id, { status: "todo" })
                                : taskStore.complete(sub.id);
                            }}
                            onkeydown={(e) => {
                              if (e.key === "Enter" || e.key === " ") {
                                e.preventDefault();
                                e.stopPropagation();
                                sub.status === "done"
                                  ? taskStore.update(sub.id, { status: "todo" })
                                  : taskStore.complete(sub.id);
                              }
                            }}
                            class="flex h-4 w-4 shrink-0 items-center justify-center rounded-full border-[1.5px] transition-colors duration-100
                              {sub.status === 'done' ? 'border-success bg-success text-white' : 'border-border-light text-transparent hover:border-text-muted hover:text-text-muted/50'}"
                          >
                            <Icon src={FiCheck} size="9" />
                          </span>

                          <span class="flex min-w-0 flex-1 flex-col gap-0.5">
                            <span class="flex items-center gap-1.5">
                              {#if sub.priority && sub.status !== "done"}
                                <span class="h-1.5 w-1.5 shrink-0 rounded-full {priorityColor(sub.priority)}"></span>
                              {/if}
                              <span
                                class="truncate text-[12px] {sub.status === 'done' ? 'text-text-muted line-through' : 'text-text-primary'}"
                              >
                                {sub.title}
                              </span>
                              {#if sub.status === "doing"}
                                <span class="shrink-0 rounded-full bg-accent/12 px-1 py-0.5 text-[9px] font-medium text-accent">
                                  In Progress
                                </span>
                              {/if}
                            </span>
                            <span class="flex items-center gap-2">
                              {#if sub.due}
                                <span class="flex items-center gap-0.5 text-[10px] {isOverdue(sub.due) && sub.status !== 'done' ? 'text-error' : 'text-text-muted'}">
                                  <Icon src={FiCalendar} size="9" />
                                  {formatDate(sub.due)}
                                </span>
                              {/if}
                            </span>
                          </span>
                        </button>
                      </TaskContextMenu>
                      <div class="ml-10 mr-4 border-b border-border/30 md:ml-12 md:mr-5"></div>
                    </div>
                  {/each}
                  <!-- Add subtask row -->
                  <button
                    onclick={() => openAdd(task.id)}
                    class="flex w-full items-center gap-3 pl-10 pr-4 py-1.5 text-left text-[12px] text-text-muted hover:text-text-secondary transition-colors md:pl-12 md:pr-5"
                  >
                    <Icon src={FiCornerDownRight} size="10" className="text-text-muted/30" />
                    <Icon src={FiPlus} size="10" />
                    <span>Add subtask</span>
                  </button>
                  <div class="mx-4 border-b border-border/30 md:mx-5"></div>
                {/if}
              {/each}
            </div>
          {/if}
        </div>
      {:else}
        <!-- Kanban View -->
        <div class="flex flex-1 gap-3 overflow-x-auto p-3">
          {#each kanbanCols as col (col.key)}
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
                  <TaskContextMenu {task} onOpenDetail={openDetail} onAddSubtask={openAdd}>
                    <div
                      draggable="true"
                      role="button"
                      tabindex="0"
                      ondragstart={(e) => handleDragStart(e, task.id)}
                      ondragend={handleDragEnd}
                      ondragover={(e) => handleTaskDragOver(e, task.id)}
                      ondragleave={(e) => handleTaskDragLeave(e, task.id)}
                      onclick={() => openDetail(task)}
                      onkeydown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); openDetail(task); } }}
                      class="group relative flex cursor-pointer flex-col gap-1.5 rounded-lg border bg-bg p-2.5 outline-none transition-all duration-150 {draggingId === task.id ? 'border-border scale-[0.97] cursor-grabbing opacity-40' : panelOpen && taskStore.selectedId === task.id ? 'border-accent shadow-sm' : 'border-border hover:border-border-light hover:shadow-sm'}"
                    >
                      {#if dragOverTaskId === task.id && draggingId !== task.id}
                        <div class="pointer-events-none absolute inset-x-0 z-10 h-0.5 bg-accent {dragOverBefore ? '-top-1' : '-bottom-1'}"></div>
                      {/if}
                      <div class="flex items-start justify-between gap-1.5">
                        <span class="text-[12px] leading-snug text-text-primary {task.status === 'done' ? 'line-through text-text-muted' : ''}">{task.title}</span>
                        {#if task.priority}
                          <span class="mt-0.5 h-2 w-2 shrink-0 rounded-full {priorityColor(task.priority)}" title="{task.priority} priority"></span>
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
                            {@const tc = tagColorStore.get(tag)}
                            {@const tagLabel = tag.includes("::") ? tag.split("::").pop() : tag}
                            <span class="flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[10px]"
                              style={tc ? `background:${tc}20;color:${tc}` : ""}
                              class:bg-overlay-light={!tc}
                              class:text-text-muted={!tc}
                            >{tagLabel}</span>
                          {/each}
                        {/if}
                      </div>
                    </div>
                  </TaskContextMenu>
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
          {#if panelMode === "add"}
            <form onsubmit={handleAddSubmit} class="flex flex-1 flex-col overflow-hidden">
              <div class="flex items-center justify-between border-b border-border px-4 py-3">
                <h2 class="text-[14px] font-semibold text-text-primary">New Task</h2>
                <button
                  type="button"
                  onclick={closePanel}
                  class="flex h-7 w-7 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-text-secondary"
                >
                  <Icon src={FiX} size="14" />
                </button>
              </div>
              {@render detailPanel(false)}
              <div class="border-t border-border px-4 py-3">
                <button
                  type="submit"
                  disabled={!panelTitle.trim()}
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
                    onclick={closePanel}
                    class="flex h-7 w-7 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-text-secondary"
                  >
                    <Icon src={FiX} size="14" />
                  </button>
                </div>
              </div>
              {@render detailPanel(false)}
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </div>

  <!-- Mobile FAB -->
  {#if !panelOpen}
    <button
      onclick={() => openAdd()}
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
            onclick={() => { taskStore.toggleTag(tag); showSidebar = false; }}
            class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-[14px] transition-colors
              {taskStore.filterTags.includes(tag) ? 'bg-accent/10 text-accent' : 'text-text-secondary hover:bg-overlay-light'}"
          >
            <Icon src={FiTag} size="14" className={taskStore.filterTags.includes(tag) ? 'text-accent' : 'text-text-muted'} />
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
    {#if panelMode === "add"}
      <form onsubmit={handleAddSubmit} class="flex flex-1 flex-col overflow-hidden">
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
          <button
            type="submit"
            disabled={!panelTitle.trim()}
            class="flex h-9 items-center rounded-lg bg-accent px-4 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40"
          >
            Add
          </button>
        </div>
        {@render detailPanel(true)}
      </form>
    {:else}
      <div class="flex flex-1 flex-col overflow-hidden">
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
          <button
            type="button"
            onclick={handleDelete}
            class="flex h-9 w-9 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-danger"
          >
            <Icon src={FiTrash2} size="17" />
          </button>
        </div>
        {@render detailPanel(true)}
      </div>
    {/if}
  </div>
{/if}

{#if colorPickerTag}
  {@const tc = tagColorStore.get(colorPickerTag)}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    data-color-picker
    class="fixed z-[100] grid grid-cols-5 gap-1.5 rounded-lg border border-border bg-bg-secondary p-2 shadow-elevated"
    style="left:{colorPickerPos.x}px;top:{colorPickerPos.y}px"
    onclick={(e) => e.stopPropagation()}
  >
    {#each tagColorStore.palette as color}
      <button
        onclick={() => { tagColorStore.set(colorPickerTag!, color); colorPickerTag = null; }}
        class="h-5 w-5 rounded-full border-2 transition-transform hover:scale-110 {tc === color ? 'border-white' : 'border-transparent'}"
        style="background:{color}"
      ></button>
    {/each}
    {#if tc}
      <button
        onclick={() => { tagColorStore.remove(colorPickerTag!); colorPickerTag = null; }}
        class="flex h-5 w-5 items-center justify-center rounded-full border border-border text-text-muted hover:border-border-light hover:text-text-secondary"
        title="Remove color"
      >
        <Icon src={FiX} size="10" />
      </button>
    {/if}
  </div>
{/if}

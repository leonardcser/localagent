import {
  getTasks,
  createTask,
  updateTask,
  completeTask,
  deleteTask,
  type Task,
} from "$lib/api";

const PREFS_KEY = "tasks-prefs";

export type SmartList =
  | "all"
  | "today"
  | "tomorrow"
  | "next7"
  | "overdue"
  | "inbox"
  | "done";

interface TaskPrefs {
  view: "list" | "kanban";
  smartList: SmartList;
  filterTags: string[];
  selectedId: string;
}

function loadPrefs(): TaskPrefs {
  try {
    const raw = localStorage.getItem(PREFS_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      return {
        view: parsed.view ?? "list",
        smartList: parsed.smartList ?? "all",
        filterTags:
          parsed.filterTags ?? (parsed.filterTag ? [parsed.filterTag] : []),
        selectedId: parsed.selectedId ?? "",
      };
    }
  } catch {
    // ignore
  }
  return {
    view: "list",
    smartList: "all",
    filterTags: [],
    selectedId: "",
  };
}

function savePrefs(prefs: TaskPrefs) {
  try {
    localStorage.setItem(PREFS_KEY, JSON.stringify(prefs));
  } catch {
    // ignore
  }
}

function dueDatePart(due: string): string {
  return due.includes("T") ? due.split("T")[0] : due;
}

function todayStr(): string {
  return new Date().toISOString().slice(0, 10);
}

function tomorrowStr(): string {
  return new Date(Date.now() + 86400000).toISOString().slice(0, 10);
}

function next7Str(): string {
  return new Date(Date.now() + 7 * 86400000).toISOString().slice(0, 10);
}

function createTaskStore() {
  const initialPrefs = loadPrefs();
  let tasks = $state<Task[]>([]);
  let loading = $state(false);
  let search = $state("");
  let smartList = $state<SmartList>(initialPrefs.smartList);
  let filterTags = $state<string[]>(initialPrefs.filterTags);
  let view = $state<"list" | "kanban">(initialPrefs.view);
  let selectedId = $state(initialPrefs.selectedId);

  function persistPrefs() {
    savePrefs({ view, smartList, filterTags, selectedId });
  }

  let allTags = $derived.by(() => {
    const set = new Set<string>();
    for (const t of tasks) {
      for (const tag of t.tags ?? []) set.add(tag);
    }
    return [...set].sort();
  });

  let counts = $derived.by(() => {
    const today = todayStr();
    const tomorrow = tomorrowStr();
    const next7 = next7Str();
    let all = 0;
    let todayCount = 0;
    let tomorrowCount = 0;
    let next7Count = 0;
    let overdue = 0;
    let inbox = 0;
    let done = 0;
    for (const t of tasks) {
      if (t.status === "done") {
        done++;
        continue;
      }
      all++;
      if (t.due) {
        const dp = dueDatePart(t.due);
        if (dp < today) overdue++;
        if (dp <= today) todayCount++;
        if (dp === tomorrow) tomorrowCount++;
        if (dp <= next7) next7Count++;
      } else {
        inbox++;
      }
    }
    return {
      all,
      today: todayCount,
      tomorrow: tomorrowCount,
      next7: next7Count,
      overdue,
      inbox,
      done,
    };
  });

  function getPriorityValue(priority?: string): number {
    switch (priority) {
      case "high":
        return 1;
      case "medium":
        return 2;
      case "low":
        return 3;
      default:
        return 4;
    }
  }

  function sortTasks(a: Task, b: Task): number {
    const aDone = a.status === "done" ? 1 : 0;
    const bDone = b.status === "done" ? 1 : 0;
    if (aDone !== bDone) return aDone - bDone;
    const pDiff = getPriorityValue(a.priority) - getPriorityValue(b.priority);
    if (pDiff !== 0) return pDiff;
    return (a.order ?? 0) - (b.order ?? 0);
  }

  function applyDateFilter(t: Task): boolean {
    switch (smartList) {
      case "today":
        return !!t.due && dueDatePart(t.due) <= todayStr();
      case "tomorrow":
        return !!t.due && dueDatePart(t.due) === tomorrowStr();
      case "next7":
        return !!t.due && dueDatePart(t.due) <= next7Str();
      case "overdue":
        return !!t.due && dueDatePart(t.due) < todayStr();
      case "inbox":
        return !t.due;
      case "done":
        return true;
      case "all":
        return true;
    }
  }

  function applyTagFilter(result: Task[]): Task[] {
    if (filterTags.length === 0) return result;
    const matchingParentIds = new Set<string>();
    for (const t of tasks) {
      if (t.parentId && filterTags.every((tag) => t.tags?.includes(tag))) {
        matchingParentIds.add(t.parentId);
      }
    }
    return result.filter(
      (t) =>
        filterTags.every((tag) => t.tags?.includes(tag)) ||
        matchingParentIds.has(t.id),
    );
  }

  let filtered = $derived.by(() => {
    let result = tasks;

    if (search) {
      const q = search.toLowerCase();
      result = result.filter(
        (t) =>
          t.title.toLowerCase().includes(q) ||
          (t.description?.toLowerCase().includes(q) ?? false),
      );
      return result;
    }

    if (smartList === "done") {
      result = result.filter((t) => t.status === "done" && applyDateFilter(t));
    } else {
      result = result.filter(
        (t) => t.status !== "done" && applyDateFilter(t),
      );
    }

    result = applyTagFilter(result);
    return [...result].sort(sortTasks);
  });

  let completedFiltered = $derived.by(() => {
    if (smartList === "done" || search) return [];
    let result = tasks.filter(
      (t) => t.status === "done" && applyDateFilter(t),
    );
    result = applyTagFilter(result);
    return [...result].sort((a, b) => (a.order ?? 0) - (b.order ?? 0));
  });

  let kanbanColumns = $derived.by(() => {
    return {
      todo: filtered.filter((t) => t.status === "todo"),
      doing: filtered.filter((t) => t.status === "doing"),
      done: filtered.filter((t) => t.status === "done"),
    };
  });

  let childrenMap = $derived.by(() => {
    const map = new Map<string, Task[]>();
    for (const t of tasks) {
      if (t.parentId) {
        const list = map.get(t.parentId) ?? [];
        list.push(t);
        map.set(t.parentId, list);
      }
    }
    return map;
  });

  function subtasksOf(id: string): Task[] {
    const subs = childrenMap.get(id) ?? [];
    return [...subs].sort((a, b) => {
      const aDone = a.status === "done" ? 1 : 0;
      const bDone = b.status === "done" ? 1 : 0;
      if (aDone !== bDone) return aDone - bDone;
      return (a.order ?? 0) - (b.order ?? 0);
    });
  }

  function isParent(id: string): boolean {
    return childrenMap.has(id);
  }

  let topLevelFiltered = $derived.by(() => {
    return filtered.filter((t) => !t.parentId);
  });

  let topLevelCompletedFiltered = $derived.by(() => {
    return completedFiltered.filter((t) => !t.parentId);
  });

  async function load() {
    loading = true;
    tasks = await getTasks();
    loading = false;
  }

  async function add(task: Partial<Task>) {
    const created = await createTask(task);
    if (created) {
      tasks = [...tasks, created];
    }
    return created;
  }

  async function update(id: string, patch: Partial<Task>) {
    const updated = await updateTask(id, patch);
    if (updated) {
      tasks = tasks.map((t) => (t.id === id ? updated : t));
    }
    return updated;
  }

  async function complete(id: string) {
    const completed = await completeTask(id);
    if (completed) {
      tasks = tasks.map((t) => (t.id === id ? completed : t));
      await load();
    }
    return completed;
  }

  async function remove(id: string) {
    const ok = await deleteTask(id);
    if (ok) {
      tasks = tasks.filter((t) => t.id !== id);
    }
    return ok;
  }

  function applyEvent(action: string, task: Task) {
    switch (action) {
      case "created":
        if (!tasks.some((t) => t.id === task.id)) {
          tasks = [...tasks, task];
        }
        break;
      case "updated":
        tasks = tasks.map((t) => (t.id === task.id ? task : t));
        break;
      case "deleted":
        tasks = tasks.filter((t) => t.id !== task.id && t.parentId !== task.id);
        break;
    }
  }

  async function reorder(id: string, newOrder: number) {
    return update(id, { order: newOrder } as Partial<Task>);
  }

  async function moveStatus(id: string, status: string) {
    if (status === "done") {
      return complete(id);
    }
    return update(id, { status } as Partial<Task>);
  }

  return {
    get tasks() {
      return tasks;
    },
    get loading() {
      return loading;
    },
    get search() {
      return search;
    },
    set search(v: string) {
      search = v;
    },
    get smartList() {
      return smartList;
    },
    set smartList(v: SmartList) {
      smartList = v;
      persistPrefs();
    },
    get filterTags() {
      return filterTags;
    },
    set filterTags(v: string[]) {
      filterTags = v;
      persistPrefs();
    },
    toggleTag(tag: string, multi = false) {
      if (filterTags.includes(tag)) {
        filterTags = filterTags.filter((t) => t !== tag);
      } else if (multi) {
        filterTags = [...filterTags, tag];
      } else {
        filterTags = [tag];
      }
      persistPrefs();
    },
    get view() {
      return view;
    },
    set view(v: "list" | "kanban") {
      view = v;
      persistPrefs();
    },
    get allTags() {
      return allTags;
    },
    get filtered() {
      return filtered;
    },
    get kanbanColumns() {
      return kanbanColumns;
    },
    get selectedId() {
      return selectedId;
    },
    set selectedId(v: string) {
      selectedId = v;
      persistPrefs();
    },
    get counts() {
      return counts;
    },
    get topLevelFiltered() {
      return topLevelFiltered;
    },
    get completedFiltered() {
      return completedFiltered;
    },
    get topLevelCompletedFiltered() {
      return topLevelCompletedFiltered;
    },
    subtasksOf,
    isParent,
    load,
    add,
    update,
    complete,
    remove,
    moveStatus,
    reorder,
    applyEvent,
  };
}

export const taskStore = createTaskStore();

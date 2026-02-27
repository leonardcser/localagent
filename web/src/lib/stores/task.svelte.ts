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
	| "inbox"
	| "done";

interface TaskPrefs {
	view: "list" | "kanban";
	smartList: SmartList;
	filterTag: string;
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
				filterTag: parsed.filterTag ?? "",
				selectedId: parsed.selectedId ?? "",
			};
		}
	} catch {
		// ignore
	}
	return {
		view: "list",
		smartList: "all",
		filterTag: "",
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
	let filterTag = $state(initialPrefs.filterTag);
	let view = $state<"list" | "kanban">(initialPrefs.view);
	let selectedId = $state(initialPrefs.selectedId);

	function persistPrefs() {
		savePrefs({ view, smartList, filterTag, selectedId });
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
		let inbox = 0;
		let done = 0;
		for (const t of tasks) {
			if (t.status === "done") {
				done++;
				continue;
			}
			all++;
			if (t.due) {
				if (t.due <= today) todayCount++;
				if (t.due === tomorrow) tomorrowCount++;
				if (t.due <= next7) next7Count++;
			} else {
				inbox++;
				todayCount++;
			}
		}
		return {
			all,
			today: todayCount,
			tomorrow: tomorrowCount,
			next7: next7Count,
			inbox,
			done,
		};
	});

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

		switch (smartList) {
			case "today": {
				const today = todayStr();
				result = result.filter(
					(t) => t.status !== "done" && (!t.due || t.due <= today),
				);
				break;
			}
			case "tomorrow": {
				const tomorrow = tomorrowStr();
				result = result.filter(
					(t) => t.status !== "done" && t.due === tomorrow,
				);
				break;
			}
			case "next7": {
				const next7 = next7Str();
				result = result.filter(
					(t) => t.status !== "done" && t.due && t.due <= next7,
				);
				break;
			}
			case "inbox":
				result = result.filter((t) => t.status !== "done" && !t.due);
				break;
			case "done":
				result = result.filter((t) => t.status === "done");
				break;
			case "all":
				result = result.filter((t) => t.status !== "done");
				break;
		}

		if (filterTag) {
			result = result.filter((t) => t.tags?.includes(filterTag));
		}

		return result;
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
		return childrenMap.get(id) ?? [];
	}

	function isParent(id: string): boolean {
		return childrenMap.has(id);
	}

	let topLevelFiltered = $derived.by(() => {
		return filtered.filter((t) => !t.parentId);
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
		get filterTag() {
			return filterTag;
		},
		set filterTag(v: string) {
			filterTag = v;
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
		subtasksOf,
		isParent,
		load,
		add,
		update,
		complete,
		remove,
		moveStatus,
	};
}

export const taskStore = createTaskStore();

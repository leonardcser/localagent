import {
	getTasks,
	createTask,
	updateTask,
	completeTask,
	deleteTask,
	type Task,
} from "$lib/api";

const PREFS_KEY = "tasks-prefs";

interface TaskPrefs {
	view: "list" | "kanban";
	filterStatus: string;
	filterTag: string;
	filterDue: string;
}

function loadPrefs(): TaskPrefs {
	try {
		const raw = localStorage.getItem(PREFS_KEY);
		if (raw) return JSON.parse(raw);
	} catch {
		// ignore
	}
	return { view: "list", filterStatus: "", filterTag: "", filterDue: "" };
}

function savePrefs(prefs: TaskPrefs) {
	try {
		localStorage.setItem(PREFS_KEY, JSON.stringify(prefs));
	} catch {
		// ignore
	}
}

function createTaskStore() {
	const initialPrefs = loadPrefs();
	let tasks = $state<Task[]>([]);
	let loading = $state(false);
	let search = $state("");
	let filterStatus = $state(initialPrefs.filterStatus);
	let filterTag = $state(initialPrefs.filterTag);
	let filterDue = $state(initialPrefs.filterDue);
	let view = $state<"list" | "kanban">(initialPrefs.view);

	function persistPrefs() {
		savePrefs({
			view,
			filterStatus,
			filterTag,
			filterDue,
		});
	}

	let allTags = $derived.by(() => {
		const set = new Set<string>();
		for (const t of tasks) {
			for (const tag of t.tags ?? []) set.add(tag);
		}
		return [...set].sort();
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
		}

		if (filterStatus) {
			result = result.filter((t) => t.status === filterStatus);
		}

		if (filterTag) {
			result = result.filter((t) => t.tags?.includes(filterTag));
		}

		if (filterDue) {
			result = result.filter((t) => t.due === filterDue);
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
		get filterStatus() {
			return filterStatus;
		},
		set filterStatus(v: string) {
			filterStatus = v;
			persistPrefs();
		},
		get filterTag() {
			return filterTag;
		},
		set filterTag(v: string) {
			filterTag = v;
			persistPrefs();
		},
		get filterDue() {
			return filterDue;
		},
		set filterDue(v: string) {
			filterDue = v;
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
		load,
		add,
		update,
		complete,
		remove,
		moveStatus,
	};
}

export const taskStore = createTaskStore();

import {
  getSlots,
  createSlot,
  updateSlot,
  deleteSlot,
  type Slot,
} from "$lib/api";

function createSlotStore() {
  let slots = $state<Slot[]>([]);
  let loading = $state(false);

  async function load(start?: number, end?: number) {
    loading = true;
    slots = await getSlots({ start, end });
    loading = false;
  }

  async function loadForTask(taskId: string) {
    loading = true;
    slots = await getSlots({ taskId });
    loading = false;
  }

  async function add(slot: Partial<Slot>) {
    const created = await createSlot(slot);
    if (created) {
      slots = [...slots, created];
    }
    return created;
  }

  async function update(id: string, patch: Partial<Slot>) {
    const updated = await updateSlot(id, patch);
    if (updated) {
      slots = slots.map((s) => (s.id === id ? updated : s));
    }
    return updated;
  }

  async function remove(id: string) {
    const ok = await deleteSlot(id);
    if (ok) {
      slots = slots.filter((s) => s.id !== id);
    }
    return ok;
  }

  function applyEvent(action: string, slot: Slot) {
    switch (action) {
      case "created":
        if (!slots.some((s) => s.id === slot.id)) {
          slots = [...slots, slot];
        }
        break;
      case "updated":
        slots = slots.map((s) => (s.id === slot.id ? slot : s));
        break;
      case "deleted":
        slots = slots.filter((s) => s.id !== slot.id);
        break;
    }
  }

  function forTask(taskId: string): Slot[] {
    return slots.filter((s) => s.taskId === taskId);
  }

  return {
    get slots() {
      return slots;
    },
    get loading() {
      return loading;
    },
    load,
    loadForTask,
    add,
    update,
    remove,
    applyEvent,
    forTask,
  };
}

export const slotStore = createSlotStore();

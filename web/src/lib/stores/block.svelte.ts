import {
  getBlocks,
  createBlock,
  updateBlock,
  deleteBlock,
  type Block,
} from "$lib/api";

function createBlockStore() {
  let blocks = $state<Block[]>([]);
  let loading = $state(false);

  async function load(start?: number, end?: number) {
    loading = true;
    blocks = await getBlocks({ start, end });
    loading = false;
  }

  async function loadForTask(taskId: string) {
    loading = true;
    blocks = await getBlocks({ taskId });
    loading = false;
  }

  async function add(block: Partial<Block>) {
    const created = await createBlock(block);
    if (created) {
      blocks = [...blocks, created];
    }
    return created;
  }

  async function update(id: string, patch: Partial<Block>) {
    const updated = await updateBlock(id, patch);
    if (updated) {
      blocks = blocks.map((b) => (b.id === id ? updated : b));
    }
    return updated;
  }

  async function remove(id: string) {
    const ok = await deleteBlock(id);
    if (ok) {
      blocks = blocks.filter((b) => b.id !== id);
    }
    return ok;
  }

  function applyEvent(action: string, block: Block) {
    switch (action) {
      case "created":
        if (!blocks.some((b) => b.id === block.id)) {
          blocks = [...blocks, block];
        }
        break;
      case "updated":
        blocks = blocks.map((b) => (b.id === block.id ? block : b));
        break;
      case "deleted":
        blocks = blocks.filter((b) => b.id !== block.id);
        break;
    }
  }

  function forTask(taskId: string): Block[] {
    return blocks.filter((b) => b.taskId === taskId);
  }

  return {
    get blocks() {
      return blocks;
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

export const blockStore = createBlockStore();

import {
  getLinks,
  createLink,
  updateLink,
  deleteLink,
  type Link,
} from "$lib/api";

function createLinkStore() {
  let links = $state<Link[]>([]);
  let loading = $state(false);
  let filterTags = $state<string[]>([]);

  async function load() {
    loading = true;
    links = await getLinks();
    loading = false;
  }

  async function add(link: Partial<Link>) {
    const created = await createLink(link);
    if (created) {
      links = [created, ...links];
    }
    return created;
  }

  async function update(id: string, patch: Partial<Link>) {
    const updated = await updateLink(id, patch);
    if (updated) {
      links = links.map((l) => (l.id === id ? updated : l));
    }
    return updated;
  }

  async function remove(id: string) {
    const ok = await deleteLink(id);
    if (ok) {
      links = links.filter((l) => l.id !== id);
    }
    return ok;
  }

  function applyEvent(action: string, link: Link) {
    switch (action) {
      case "created":
        if (!links.some((l) => l.id === link.id)) {
          links = [link, ...links];
        }
        break;
      case "updated":
        links = links.map((l) => (l.id === link.id ? link : l));
        break;
      case "deleted":
        links = links.filter((l) => l.id !== link.id);
        break;
    }
  }

  let allTags = $derived.by(() => {
    const set = new Set<string>();
    for (const l of links) {
      for (const t of l.tags ?? []) set.add(t);
    }
    return [...set].sort();
  });

  return {
    get links() {
      return links;
    },
    get loading() {
      return loading;
    },
    get allTags() {
      return allTags;
    },
    get filterTags() {
      return filterTags;
    },
    set filterTags(v: string[]) {
      filterTags = v;
    },
    toggleTag(tag: string, multi = false) {
      if (filterTags.includes(tag)) {
        filterTags = filterTags.filter((t) => t !== tag);
      } else if (multi) {
        filterTags = [...filterTags, tag];
      } else {
        filterTags = [tag];
      }
    },
    load,
    add,
    update,
    remove,
    applyEvent,
  };
}

export const linkStore = createLinkStore();

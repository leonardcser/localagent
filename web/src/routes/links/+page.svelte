<script lang="ts">
import { onMount } from "svelte";
import { linkStore } from "$lib/stores/link.svelte";
import { tagColorStore } from "$lib/stores/tagColor.svelte";
import type { Link } from "$lib/api";
import { ContextMenu } from "bits-ui";
import { Icon } from "svelte-icons-pack";
import {
  FiPlus,
  FiTrash2,
  FiSearch,
  FiX,
  FiExternalLink,
  FiTag,
  FiArrowLeft,
  FiLink,
  FiChevronRight,
  FiEdit2,
  FiCopy,
} from "svelte-icons-pack/fi";
import TagTree from "$lib/components/TagTree.svelte";
import TagColorPicker from "$lib/components/TagColorPicker.svelte";

// --- State ---
let searchQuery = $state("");
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

function closeColorPicker(e: MouseEvent) {
  if (
    colorPickerTag &&
    !(e.target as HTMLElement).closest("[data-color-picker]")
  ) {
    colorPickerTag = null;
  }
}

let panelOpen = $state(false);
let editId = $state<string | null>(null);

let formUrl = $state("");
let formTitle = $state("");
let formDescription = $state("");
let formTags = $state("");

onMount(async () => {
  await linkStore.load();
});

// --- Derived ---
let filteredLinks = $derived.by(() => {
  let list = linkStore.links;
  if (linkStore.filterTags.length > 0) {
    list = list.filter((l) =>
      linkStore.filterTags.every((tag) => l.tags?.includes(tag)),
    );
  }
  if (searchQuery.trim()) {
    const q = searchQuery.toLowerCase();
    list = list.filter(
      (l) =>
        l.title?.toLowerCase().includes(q) ||
        l.url.toLowerCase().includes(q) ||
        l.description?.toLowerCase().includes(q),
    );
  }
  return list;
});

// --- Helpers ---
function hostname(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, "");
  } catch {
    return url;
  }
}

function faviconUrl(url: string): string {
  try {
    const host = new URL(url).hostname;
    return `https://www.google.com/s2/favicons?domain=${host}&sz=32`;
  } catch {
    return "";
  }
}

function relativeDate(ms: number): string {
  const diff = Date.now() - ms;
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(ms).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
  });
}

// --- Panel ---
function openAdd() {
  editId = null;
  formUrl = "";
  formTitle = "";
  formDescription = "";
  formTags = "";
  panelOpen = true;
}

function openEdit(link: Link) {
  editId = link.id;
  formUrl = link.url;
  formTitle = link.title ?? "";
  formDescription = link.description ?? "";
  formTags = (link.tags ?? []).join(", ");
  panelOpen = true;
}

function closePanel() {
  panelOpen = false;
  editId = null;
}

function parseTags(raw: string): string[] {
  return raw
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean);
}

// Auto-save for edit mode
let saveTimer: ReturnType<typeof setTimeout> | null = null;
function autoSave(patch: Partial<Link>) {
  if (!editId) return;
  const id = editId;
  if (saveTimer) clearTimeout(saveTimer);
  saveTimer = setTimeout(() => {
    linkStore.update(id, patch);
  }, 400);
}

// Add mode submit
async function handleAddSubmit(e: SubmitEvent) {
  e.preventDefault();
  if (!formUrl.trim()) return;
  const tags = parseTags(formTags);
  await linkStore.add({
    url: formUrl.trim(),
    title: formTitle.trim() || undefined,
    description: formDescription.trim() || undefined,
    tags: tags.length ? tags : undefined,
  });
  closePanel();
}

async function removeLink(id: string, e?: MouseEvent) {
  e?.stopPropagation();
  await linkStore.remove(id);
  if (panelOpen && editId === id) closePanel();
}

async function copyUrl(url: string) {
  await navigator.clipboard.writeText(url);
}
</script>


<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div class="flex h-full overflow-hidden" onclick={closeColorPicker}>
  <!-- Desktop sidebar -->
  <aside class="hidden w-52 shrink-0 flex-col border-r border-border bg-bg md:flex">
    <div class="flex flex-col px-1.5 py-2">
      <span
        class="px-2 pb-1 text-[10px] font-semibold uppercase tracking-widest text-text-muted"
        >Filter</span
      >
      <button
        onclick={() => (linkStore.filterTags = [])}
        class="flex items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-[13px] transition-colors
          {linkStore.filterTags.length === 0
          ? 'bg-accent/10 text-accent'
          : 'text-text-secondary hover:bg-overlay-light hover:text-text-primary'}"
      >
        <Icon
          src={FiLink}
          size="13"
          className="shrink-0 {linkStore.filterTags.length === 0 ? 'text-accent' : 'text-text-muted'}"
        />
        <span class="flex-1 text-left">All links</span>
        <span
          class="min-w-5 text-right text-[11px] tabular-nums {linkStore.filterTags.length === 0 ? 'text-accent/70' : 'text-text-muted'}"
          >{linkStore.links.length}</span
        >
      </button>
    </div>

    {#if linkStore.allTags.length > 0}
      <div class="flex flex-col border-t border-border px-1.5 py-2">
        <span
          class="px-2 pb-1 text-[10px] font-semibold uppercase tracking-widest text-text-muted"
          >Tags</span
        >
        <TagTree
          tags={linkStore.allTags}
          filterTags={linkStore.filterTags}
          ontoggle={(tag, multi) => linkStore.toggleTag(tag, multi)}
          onopencolorpicker={openColorPicker}
        />
      </div>
    {/if}
  </aside>

  <!-- Mobile sidebar overlay -->
  {#if showSidebar}
    <div
      class="fixed inset-0 z-30 bg-black/20 backdrop-blur-[1px] md:hidden"
      role="presentation"
      onclick={() => (showSidebar = false)}
      onkeydown={() => {}}
    ></div>
    <aside
      class="fixed left-0 top-0 z-40 flex h-full w-64 flex-col border-r border-border bg-bg-secondary shadow-elevated md:hidden"
      style="padding-top: max(env(safe-area-inset-top, 0px), 10px)"
    >
      <div class="flex items-center justify-between border-b border-border px-3 py-2">
        <span class="text-[14px] font-semibold text-text-primary"
          >Filter</span
        >
        <button
          onclick={() => (showSidebar = false)}
          class="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light"
        >
          <Icon src={FiX} size="16" />
        </button>
      </div>
      <div class="flex flex-col gap-0.5 overflow-y-auto px-2 py-2">
        <button
          onclick={() => {
            linkStore.filterTags = [];
            showSidebar = false;
          }}
          class="flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-[14px] transition-colors
            {linkStore.filterTags.length === 0
            ? 'bg-accent/10 text-accent'
            : 'text-text-secondary hover:bg-overlay-light'}"
        >
          <Icon src={FiLink} size="14" />
          All links
        </button>
        <TagTree
          tags={linkStore.allTags}
          filterTags={linkStore.filterTags}
          ontoggle={(tag, multi) => linkStore.toggleTag(tag, multi)}
          onopencolorpicker={openColorPicker}
        />
      </div>
    </aside>
  {/if}

  <!-- Main area -->
  <div class="flex flex-1 flex-col overflow-hidden">
    <!-- Mobile header -->
    <div
      class="flex shrink-0 items-center gap-2 border-b border-border px-3 py-2 md:hidden"
    >
      <button
        onclick={() => (showSidebar = !showSidebar)}
        class="flex h-8 items-center gap-1.5 rounded-lg px-2 text-[13px] font-medium text-text-primary hover:bg-overlay-light"
      >
        {linkStore.filterTags.length > 0
          ? `${linkStore.filterTags.length} tag${linkStore.filterTags.length > 1 ? "s" : ""}`
          : "All links"}
        <Icon
          src={FiChevronRight}
          size="14"
          className="text-text-muted rotate-90"
        />
      </button>
      <div class="ml-auto flex items-center gap-1">
        <button
          onclick={() => (showSearch = !showSearch)}
          class="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-text-secondary"
        >
          <Icon src={FiSearch} size="15" />
        </button>
        <button
          onclick={openAdd}
          class="flex h-8 w-8 items-center justify-center rounded-lg bg-accent text-white hover:opacity-90"
        >
          <Icon src={FiPlus} size="15" />
        </button>
      </div>
    </div>

    {#if showSearch}
      <div
        class="flex items-center gap-2 border-b border-border px-3 py-2 md:hidden"
      >
        <div class="relative flex-1">
          <Icon
            src={FiSearch}
            size="14"
            className="absolute left-2.5 top-1/2 -translate-y-1/2 text-text-muted pointer-events-none"
          />
          <input
            type="text"
            placeholder="Search links…"
            bind:value={searchQuery}
            class="h-8 w-full rounded-lg border border-border bg-bg-tertiary pl-8 pr-2 text-[13px] text-text-primary outline-none focus:border-accent"
          />
        </div>
        <button
          onclick={() => {
            showSearch = false;
            searchQuery = "";
          }}
          class="text-text-muted hover:text-text-secondary"
        >
          <Icon src={FiX} size="16" />
        </button>
      </div>
    {/if}

    <!-- Desktop header -->
    <div
      class="hidden shrink-0 items-center gap-2 border-b border-border px-3 py-2 md:flex"
    >
      <div class="relative flex-1">
        <Icon
          src={FiSearch}
          size="14"
          className="absolute left-2.5 top-1/2 -translate-y-1/2 text-text-muted pointer-events-none"
        />
        <input
          type="text"
          placeholder="Search links…"
          bind:value={searchQuery}
          class="h-7 w-full rounded-md border border-border bg-bg-tertiary pl-7 pr-2 text-[12px] text-text-primary outline-none focus:border-accent"
        />
      </div>
      <button
        onclick={openAdd}
        class="flex h-7 items-center gap-1.5 rounded-md bg-accent px-2.5 text-[12px] font-medium text-white hover:opacity-90"
      >
        <Icon src={FiPlus} size="13" />
        Add
      </button>
    </div>

    <!-- Link list -->
    <div class="flex-1 overflow-y-auto p-3">
      {#if linkStore.loading}
        <div
          class="flex h-32 items-center justify-center text-[12px] text-text-muted"
        >
          Loading…
        </div>
      {:else if filteredLinks.length === 0}
        <div
          class="flex h-32 flex-col items-center justify-center gap-2 text-text-muted"
        >
          <Icon src={FiExternalLink} size="24" />
          <span class="text-[12px]">No links yet</span>
        </div>
      {:else}
        <div class="flex flex-col gap-0.5">
          {#each filteredLinks as link (link.id)}
            <ContextMenu.Root>
              <ContextMenu.Trigger class="contents">
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <div
                  class="group flex w-full cursor-pointer items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-colors hover:bg-overlay-light
                    {panelOpen && editId === link.id ? 'bg-overlay-light' : ''}"
                  onclick={() => openEdit(link)}
                >
                  <img
                    src={faviconUrl(link.url)}
                    alt=""
                    class="h-4 w-4 shrink-0 rounded-sm"
                    onerror={(e) => {
                      (e.currentTarget as HTMLImageElement).style.display =
                        "none";
                    }}
                  />
                  <div class="flex min-w-0 flex-1 flex-col gap-0.5">
                    <div class="flex items-center gap-2">
                      <span
                        class="truncate text-[13px] font-medium text-text-primary"
                        >{link.title || hostname(link.url)}</span
                      >
                      <span class="shrink-0 text-[10px] text-text-muted"
                        >{hostname(link.url)}</span
                      >
                    </div>
                    {#if link.description}
                      <span
                        class="truncate text-[11px] text-text-secondary"
                        >{link.description}</span
                      >
                    {/if}
                    {#if (link.tags ?? []).length > 0}
                      <div class="flex items-center gap-1 mt-0.5">
                        {#each link.tags ?? [] as tag}
                          {@const tc = tagColorStore.get(tag)}
                          {@const tagLabel = tag.includes("::") ? tag.split("::").pop() : tag}
                          <span
                            class="rounded-sm px-1.5 py-0.5 text-[10px] font-medium {tc ? '' : 'bg-overlay-light text-text-secondary'}"
                            style={tc ? `background:${tc}18;color:${tc}` : ''}
                            >{tagLabel}</span
                          >
                        {/each}
                        <span class="ml-1 text-[10px] text-text-muted"
                          >{relativeDate(link.createdAtMs)}</span
                        >
                      </div>
                    {:else}
                      <span class="text-[10px] text-text-muted mt-0.5"
                        >{relativeDate(link.createdAtMs)}</span
                      >
                    {/if}
                  </div>
                  <a
                    href={link.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    onclick={(e) => e.stopPropagation()}
                    class="shrink-0 text-text-muted hover:text-accent"
                    title="Open link"
                  >
                    <Icon src={FiExternalLink} size="13" />
                  </a>
                  <button
                    onclick={(e) => removeLink(link.id, e)}
                    class="hidden h-6 w-6 shrink-0 items-center justify-center rounded-md text-text-muted hover:bg-error/10 hover:text-error group-hover:flex"
                    title="Delete"
                  >
                    <Icon src={FiTrash2} size="12" />
                  </button>
                </div>
              </ContextMenu.Trigger>

              <ContextMenu.Portal>
                <ContextMenu.Content
                  class="z-50 min-w-44 rounded-lg border border-border bg-bg-secondary p-1 shadow-elevated"
                  sideOffset={5}
                >
                  <ContextMenu.Item
                    class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
                    onSelect={() => window.open(link.url, "_blank", "noopener,noreferrer")}
                  >
                    <Icon src={FiExternalLink} size="14" className="text-text-muted" />
                    Open URL
                  </ContextMenu.Item>
                  <ContextMenu.Item
                    class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
                    onSelect={() => copyUrl(link.url)}
                  >
                    <Icon src={FiCopy} size="14" className="text-text-muted" />
                    Copy URL
                  </ContextMenu.Item>
                  <ContextMenu.Item
                    class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-overlay-light data-[highlighted]:text-text-primary"
                    onSelect={() => openEdit(link)}
                  >
                    <Icon src={FiEdit2} size="14" className="text-text-muted" />
                    Edit
                  </ContextMenu.Item>
                  <ContextMenu.Separator class="mx-1 my-1 h-px bg-border" />
                  <ContextMenu.Item
                    class="flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none data-[highlighted]:bg-error/10 data-[highlighted]:text-error"
                    onSelect={() => removeLink(link.id)}
                  >
                    <Icon src={FiTrash2} size="14" />
                    Delete
                  </ContextMenu.Item>
                </ContextMenu.Content>
              </ContextMenu.Portal>
            </ContextMenu.Root>
          {/each}
        </div>
      {/if}
    </div>
  </div>

  <!-- Detail panel (desktop) -->
  {#if panelOpen}
    <div
      class="hidden w-80 shrink-0 flex-col border-l border-border bg-bg md:flex"
    >
      {#if editId}
        <!-- Edit mode: auto-save -->
        <div
          class="flex items-center justify-between border-b border-border px-4 py-3"
        >
          <h2 class="text-[14px] font-semibold text-text-primary">
            Edit link
          </h2>
          <div class="flex items-center gap-1">
            <button
              onclick={() => editId && removeLink(editId)}
              class="flex h-7 w-7 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-error"
              title="Delete link"
            >
              <Icon src={FiTrash2} size="14" />
            </button>
            <button
              onclick={closePanel}
              class="flex h-7 w-7 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-text-secondary"
            >
              <Icon src={FiX} size="14" />
            </button>
          </div>
        </div>
        <div class="flex flex-1 flex-col gap-3 overflow-y-auto p-4">
          <div>
            <label
              class="mb-1 block text-[11px] font-medium text-text-muted"
              for="link-url">URL</label
            >
            <input
              id="link-url"
              type="url"
              bind:value={formUrl}
              placeholder="https://example.com"
              class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
              onblur={() =>
                autoSave({ url: formUrl.trim() })}
            />
          </div>
          <div>
            <label
              class="mb-1 block text-[11px] font-medium text-text-muted"
              for="link-title">Title</label
            >
            <input
              id="link-title"
              type="text"
              bind:value={formTitle}
              placeholder="Optional title"
              class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
              onblur={() =>
                autoSave({
                  title: formTitle.trim(),
                })}
            />
          </div>
          <div>
            <label
              class="mb-1 block text-[11px] font-medium text-text-muted"
              for="link-desc">Description</label
            >
            <textarea
              id="link-desc"
              bind:value={formDescription}
              placeholder="Optional description"
              rows="3"
              class="w-full resize-none rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
              onblur={() =>
                autoSave({
                  description: formDescription.trim(),
                })}
            ></textarea>
          </div>
          <div>
            <label
              class="mb-1 block text-[11px] font-medium text-text-muted"
              for="link-tags">Tags</label
            >
            <input
              id="link-tags"
              type="text"
              bind:value={formTags}
              placeholder="tag1, tag2, tag3"
              class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
              onblur={() =>
                autoSave({
                  tags: parseTags(formTags),
                })}
            />
            <p class="mt-1 text-[10px] text-text-muted">
              Comma-separated
            </p>
          </div>
        </div>
      {:else}
        <!-- Add mode: form with submit -->
        <form
          onsubmit={handleAddSubmit}
          class="flex flex-1 flex-col overflow-hidden"
        >
          <div
            class="flex items-center justify-between border-b border-border px-4 py-3"
          >
            <h2
              class="text-[14px] font-semibold text-text-primary"
            >
              Add link
            </h2>
            <button
              type="button"
              onclick={closePanel}
              class="flex h-7 w-7 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-text-secondary"
            >
              <Icon src={FiX} size="14" />
            </button>
          </div>
          <div
            class="flex flex-1 flex-col gap-3 overflow-y-auto p-4"
          >
            <div>
              <label
                class="mb-1 block text-[11px] font-medium text-text-muted"
                for="link-url-add">URL *</label
              >
              <input
                id="link-url-add"
                type="url"
                bind:value={formUrl}
                placeholder="https://example.com"
                class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
              />
            </div>
            <div>
              <label
                class="mb-1 block text-[11px] font-medium text-text-muted"
                for="link-title-add">Title</label
              >
              <input
                id="link-title-add"
                type="text"
                bind:value={formTitle}
                placeholder="Optional title"
                class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
              />
            </div>
            <div>
              <label
                class="mb-1 block text-[11px] font-medium text-text-muted"
                for="link-desc-add">Description</label
              >
              <textarea
                id="link-desc-add"
                bind:value={formDescription}
                placeholder="Optional description"
                rows="3"
                class="w-full resize-none rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
              ></textarea>
            </div>
            <div>
              <label
                class="mb-1 block text-[11px] font-medium text-text-muted"
                for="link-tags-add">Tags</label
              >
              <input
                id="link-tags-add"
                type="text"
                bind:value={formTags}
                placeholder="tag1, tag2, tag3"
                class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
              />
              <p class="mt-1 text-[10px] text-text-muted">
                Comma-separated
              </p>
            </div>
          </div>
          <div class="border-t border-border px-4 py-3">
            <button
              type="submit"
              disabled={!formUrl.trim()}
              class="w-full rounded-lg bg-accent py-2 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40"
            >
              Add Link
            </button>
          </div>
        </form>
      {/if}
    </div>

    <!-- Mobile full-screen panel -->
    <div class="fixed inset-0 z-40 flex flex-col bg-bg md:hidden">
      {#if editId}
        <div
          class="flex items-center gap-2 border-b border-border px-3 py-2"
          style="padding-top: max(env(safe-area-inset-top, 0px), 10px)"
        >
          <button
            onclick={closePanel}
            class="flex h-9 w-9 items-center justify-center rounded-lg text-text-secondary hover:bg-overlay-light"
          >
            <Icon src={FiArrowLeft} size="18" />
          </button>
          <span class="flex-1"></span>
          <button
            onclick={() => editId && removeLink(editId)}
            class="flex h-9 w-9 items-center justify-center rounded-lg text-text-muted hover:bg-overlay-light hover:text-error"
          >
            <Icon src={FiTrash2} size="17" />
          </button>
        </div>
        <div class="flex flex-1 flex-col gap-4 overflow-y-auto p-4">
          <div>
            <label
              class="mb-1 block text-[12px] font-medium text-text-muted"
              for="m-link-url">URL</label
            >
            <input
              id="m-link-url"
              type="url"
              bind:value={formUrl}
              placeholder="https://example.com"
              class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-2 text-[14px] text-text-primary outline-none focus:border-accent"
              onblur={() =>
                autoSave({ url: formUrl.trim() })}
            />
          </div>
          <div>
            <label
              class="mb-1 block text-[12px] font-medium text-text-muted"
              for="m-link-title">Title</label
            >
            <input
              id="m-link-title"
              type="text"
              bind:value={formTitle}
              placeholder="Optional title"
              class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-2 text-[14px] text-text-primary outline-none focus:border-accent"
              onblur={() =>
                autoSave({
                  title: formTitle.trim(),
                })}
            />
          </div>
          <div>
            <label
              class="mb-1 block text-[12px] font-medium text-text-muted"
              for="m-link-desc">Description</label
            >
            <textarea
              id="m-link-desc"
              bind:value={formDescription}
              placeholder="Optional description"
              rows="4"
              class="w-full resize-none rounded-lg border border-border bg-bg-tertiary px-3 py-2 text-[14px] text-text-primary outline-none focus:border-accent"
              onblur={() =>
                autoSave({
                  description: formDescription.trim(),
                })}
            ></textarea>
          </div>
          <div>
            <label
              class="mb-1 block text-[12px] font-medium text-text-muted"
              for="m-link-tags">Tags</label
            >
            <input
              id="m-link-tags"
              type="text"
              bind:value={formTags}
              placeholder="tag1, tag2, tag3"
              class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-2 text-[14px] text-text-primary outline-none focus:border-accent"
              onblur={() =>
                autoSave({
                  tags: parseTags(formTags),
                })}
            />
            <p class="mt-1 text-[11px] text-text-muted">
              Comma-separated
            </p>
          </div>
        </div>
      {:else}
        <form
          onsubmit={handleAddSubmit}
          class="flex flex-1 flex-col overflow-hidden"
        >
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
              disabled={!formUrl.trim()}
              class="flex h-9 items-center rounded-lg bg-accent px-4 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40"
            >
              Add
            </button>
          </div>
          <div
            class="flex flex-1 flex-col gap-4 overflow-y-auto p-4"
          >
            <div>
              <label
                class="mb-1 block text-[12px] font-medium text-text-muted"
                for="m-link-url-add">URL *</label
              >
              <input
                id="m-link-url-add"
                type="url"
                bind:value={formUrl}
                placeholder="https://example.com"
                class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-2 text-[14px] text-text-primary outline-none focus:border-accent"
              />
            </div>
            <div>
              <label
                class="mb-1 block text-[12px] font-medium text-text-muted"
                for="m-link-title-add">Title</label
              >
              <input
                id="m-link-title-add"
                type="text"
                bind:value={formTitle}
                placeholder="Optional title"
                class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-2 text-[14px] text-text-primary outline-none focus:border-accent"
              />
            </div>
            <div>
              <label
                class="mb-1 block text-[12px] font-medium text-text-muted"
                for="m-link-desc-add">Description</label
              >
              <textarea
                id="m-link-desc-add"
                bind:value={formDescription}
                placeholder="Optional description"
                rows="4"
                class="w-full resize-none rounded-lg border border-border bg-bg-tertiary px-3 py-2 text-[14px] text-text-primary outline-none focus:border-accent"
              ></textarea>
            </div>
            <div>
              <label
                class="mb-1 block text-[12px] font-medium text-text-muted"
                for="m-link-tags-add">Tags</label
              >
              <input
                id="m-link-tags-add"
                type="text"
                bind:value={formTags}
                placeholder="tag1, tag2, tag3"
                class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-2 text-[14px] text-text-primary outline-none focus:border-accent"
              />
              <p class="mt-1 text-[11px] text-text-muted">
                Comma-separated
              </p>
            </div>
          </div>
        </form>
      {/if}
    </div>
  {/if}
</div>

{#if colorPickerTag}
  <TagColorPicker
    tag={colorPickerTag}
    position={colorPickerPos}
    onclose={() => (colorPickerTag = null)}
  />
{/if}

<script lang="ts">
import { onMount } from "svelte";
import { linkStore } from "$lib/stores/link.svelte";
import type { Link } from "$lib/api";
import { Icon } from "svelte-icons-pack";
import { FiPlus, FiTrash2, FiSearch, FiX, FiExternalLink, FiTag } from "svelte-icons-pack/fi";

// --- State ---
let searchQuery = $state("");
let activeTag = $state<string | null>(null);
let panelOpen = $state(false);
let editId = $state<string | null>(null);

let formUrl = $state("");
let formTitle = $state("");
let formDescription = $state("");
let formTags = $state("");
let formError = $state("");
let formSaving = $state(false);

onMount(async () => {
	await linkStore.load();
	const tag = new URLSearchParams(window.location.search).get("tag");
	if (tag) activeTag = tag;
});

// --- Derived ---
let allTags = $derived(linkStore.allTags());

let filteredLinks = $derived.by(() => {
	let list = linkStore.links;
	if (activeTag) list = list.filter((l) => l.tags?.includes(activeTag!));
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
	return new Date(ms).toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

function setTag(tag: string | null) {
	activeTag = tag;
	const url = new URL(window.location.href);
	if (tag) url.searchParams.set("tag", tag);
	else url.searchParams.delete("tag");
	history.replaceState({}, "", url.toString());
}

// --- Panel ---
function openAdd() {
	editId = null;
	formUrl = "";
	formTitle = "";
	formDescription = "";
	formTags = "";
	formError = "";
	panelOpen = true;
}

function openEdit(link: Link) {
	editId = link.id;
	formUrl = link.url;
	formTitle = link.title ?? "";
	formDescription = link.description ?? "";
	formTags = (link.tags ?? []).join(", ");
	formError = "";
	panelOpen = true;
}

function closePanel() {
	panelOpen = false;
	editId = null;
}

async function submitForm() {
	if (!formUrl.trim()) {
		formError = "URL is required";
		return;
	}
	formSaving = true;
	formError = "";
	const tags = formTags
		.split(",")
		.map((t) => t.trim())
		.filter(Boolean);

	if (editId) {
		await linkStore.update(editId, {
			url: formUrl.trim(),
			title: formTitle.trim(),
			description: formDescription.trim(),
			tags,
		});
	} else {
		await linkStore.add({
			url: formUrl.trim(),
			title: formTitle.trim() || undefined,
			description: formDescription.trim() || undefined,
			tags: tags.length ? tags : undefined,
		});
	}
	formSaving = false;
	closePanel();
}

async function removeLink(id: string, e: MouseEvent) {
	e.stopPropagation();
	await linkStore.remove(id);
}
</script>

<div class="flex h-full overflow-hidden">
	<!-- Sidebar: tag filter -->
	<aside class="hidden w-44 shrink-0 flex-col border-r border-border bg-bg-secondary md:flex">
		<div class="p-3 pb-2">
			<span class="text-[11px] font-semibold uppercase tracking-wider text-text-muted">Tags</span>
		</div>
		<div class="flex flex-col gap-0.5 overflow-y-auto px-2 pb-4">
			<button
				onclick={() => setTag(null)}
				class="flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-[12px] transition-colors
					{activeTag === null ? 'bg-accent text-white' : 'text-text-secondary hover:bg-overlay-light'}"
			>
				All links
			</button>
			{#each allTags as tag}
				<button
					onclick={() => setTag(tag)}
					class="flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-[12px] transition-colors
						{activeTag === tag ? 'bg-accent text-white' : 'text-text-secondary hover:bg-overlay-light'}"
				>
					<Icon src={FiTag} size="11" />
					<span class="truncate">{tag}</span>
				</button>
			{/each}
		</div>
	</aside>

	<!-- Main area -->
	<div class="flex flex-1 flex-col overflow-hidden">
		<!-- Top bar -->
		<div class="flex h-11 shrink-0 items-center gap-2 border-b border-border px-3">
			<div class="relative flex-1">
				<Icon src={FiSearch} size="14" className="absolute left-2.5 top-1/2 -translate-y-1/2 text-text-muted pointer-events-none" />
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

		<!-- Link grid -->
		<div class="flex-1 overflow-y-auto p-3">
			{#if linkStore.loading}
				<div class="flex h-32 items-center justify-center text-[12px] text-text-muted">Loading…</div>
			{:else if filteredLinks.length === 0}
				<div class="flex h-32 flex-col items-center justify-center gap-2 text-text-muted">
					<Icon src={FiExternalLink} size="24" />
					<span class="text-[12px]">No links yet</span>
				</div>
			{:else}
				<div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
					{#each filteredLinks as link (link.id)}
						<!-- svelte-ignore a11y_click_events_have_key_events -->
						<!-- svelte-ignore a11y_no_static_element_interactions -->
						<div
							class="group relative flex flex-col gap-2 rounded-lg border border-border bg-bg-secondary p-3 transition-colors hover:border-border/80 hover:bg-bg-tertiary cursor-pointer"
							onclick={() => openEdit(link)}
						>
							<!-- Delete button -->
							<button
								onclick={(e) => removeLink(link.id, e)}
								class="absolute right-2 top-2 hidden h-6 w-6 items-center justify-center rounded-md text-text-muted hover:bg-error/10 hover:text-error group-hover:flex"
								title="Delete"
							>
								<Icon src={FiTrash2} size="12" />
							</button>

							<!-- Header: favicon + title + external link -->
							<div class="flex items-start gap-2 pr-6">
								<img
									src={faviconUrl(link.url)}
									alt=""
									class="mt-0.5 h-4 w-4 shrink-0 rounded-sm"
									onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none'; }}
								/>
								<a
									href={link.url}
									target="_blank"
									rel="noopener noreferrer"
									onclick={(e) => e.stopPropagation()}
									class="flex-1 truncate text-[12px] font-semibold text-text-primary hover:text-accent"
								>
									{link.title || hostname(link.url)}
								</a>
								<a
									href={link.url}
									target="_blank"
									rel="noopener noreferrer"
									onclick={(e) => e.stopPropagation()}
									class="ml-auto shrink-0 text-text-muted hover:text-accent"
									title="Open link"
								>
									<Icon src={FiExternalLink} size="11" />
								</a>
							</div>

							<!-- Description -->
							{#if link.description}
								<p class="line-clamp-2 text-[11px] text-text-secondary">{link.description}</p>
							{/if}

							<!-- Footer: domain + date + tags -->
							<div class="flex flex-wrap items-center gap-1 mt-auto">
								<span class="text-[10px] text-text-muted">{hostname(link.url)}</span>
								<span class="text-text-muted">·</span>
								<span class="text-[10px] text-text-muted">{relativeDate(link.createdAtMs)}</span>
								{#each link.tags ?? [] as tag}
									<button
										onclick={(e) => { e.stopPropagation(); setTag(tag); }}
										class="rounded-sm px-1.5 py-0.5 text-[10px] font-medium transition-colors
											{activeTag === tag ? 'bg-accent text-white' : 'bg-overlay-light text-text-secondary hover:bg-accent hover:text-white'}"
									>
										{tag}
									</button>
								{/each}
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	</div>

	<!-- Add/Edit panel -->
	{#if panelOpen}
		<!-- Backdrop -->
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="fixed inset-0 z-30 bg-black/20 backdrop-blur-[1px]"
			onclick={closePanel}
			role="presentation"
		></div>
		<!-- Panel -->
		<div class="fixed right-0 top-0 z-40 flex h-full w-80 flex-col border-l border-border bg-bg-secondary shadow-elevated">
			<div class="flex h-11 shrink-0 items-center justify-between border-b border-border px-4">
				<span class="text-[13px] font-semibold text-text-primary">{editId ? "Edit link" : "Add link"}</span>
				<button onclick={closePanel} class="text-text-muted hover:text-text-secondary">
					<Icon src={FiX} size="16" />
				</button>
			</div>

			<div class="flex flex-1 flex-col gap-3 overflow-y-auto p-4">
				<div>
					<label class="mb-1 block text-[11px] font-medium text-text-muted" for="link-url">URL *</label>
					<input
						id="link-url"
						type="url"
						bind:value={formUrl}
						placeholder="https://example.com"
						class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
					/>
				</div>

				<div>
					<label class="mb-1 block text-[11px] font-medium text-text-muted" for="link-title">Title</label>
					<input
						id="link-title"
						type="text"
						bind:value={formTitle}
						placeholder="Optional title"
						class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
					/>
				</div>

				<div>
					<label class="mb-1 block text-[11px] font-medium text-text-muted" for="link-desc">Description</label>
					<textarea
						id="link-desc"
						bind:value={formDescription}
						placeholder="Optional description"
						rows="3"
						class="w-full resize-none rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
					></textarea>
				</div>

				<div>
					<label class="mb-1 block text-[11px] font-medium text-text-muted" for="link-tags">Tags</label>
					<input
						id="link-tags"
						type="text"
						bind:value={formTags}
						placeholder="tag1, tag2, tag3"
						class="w-full rounded-lg border border-border bg-bg-tertiary px-3 py-1.5 text-[12px] text-text-primary outline-none focus:border-accent"
					/>
					<p class="mt-1 text-[10px] text-text-muted">Comma-separated</p>
				</div>

				{#if formError}
					<p class="text-[11px] text-error">{formError}</p>
				{/if}
			</div>

			<div class="shrink-0 border-t border-border p-4">
				<button
					onclick={submitForm}
					disabled={formSaving}
					class="w-full rounded-lg bg-accent py-2 text-[12px] font-medium text-white hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
				>
					{formSaving ? "Saving…" : editId ? "Save changes" : "Add link"}
				</button>
			</div>
		</div>
	{/if}
</div>

<script module lang="ts">
export interface TagNode {
  label: string;
  path: string;
  fullTag: string | null;
  children: TagNode[];
}
</script>

<script lang="ts">
import { Icon } from "svelte-icons-pack";
import { FiChevronRight, FiTag } from "svelte-icons-pack/fi";
import { tagColorStore } from "$lib/stores/tagColor.svelte";

let {
  tags,
  filterTags,
  ontoggle,
  onopencolorpicker,
}: {
  tags: string[];
  filterTags: string[];
  ontoggle: (tag: string, multi: boolean) => void;
  onopencolorpicker: (e: MouseEvent, tag: string) => void;
} = $props();

let expandedGroups = $state(new Set<string>());

let tree = $derived.by(() => {
  const root: TagNode[] = [];
  for (const tag of tags) {
    const parts = tag.split("::");
    let level = root;
    let path = "";
    for (let i = 0; i < parts.length; i++) {
      path = path ? `${path}::${parts[i]}` : parts[i];
      let node = level.find((n) => n.label === parts[i]);
      if (!node) {
        node = { label: parts[i], path, fullTag: null, children: [] };
        level.push(node);
      }
      if (i === parts.length - 1) node.fullTag = tag;
      level = node.children;
    }
  }
  return root;
});

function isActive(node: TagNode): boolean {
  if (node.fullTag && filterTags.includes(node.fullTag)) return true;
  return node.children.some((c) => isActive(c));
}

function toggleGroup(path: string) {
  const next = new Set(expandedGroups);
  if (next.has(path)) next.delete(path);
  else next.add(path);
  expandedGroups = next;
}
</script>

{#snippet renderNodes(nodes: TagNode[], depth: number)}
	{#each nodes as node}
		{@const active = isActive(node)}
		{@const tc = node.fullTag ? tagColorStore.get(node.fullTag) : null}
		{@const hasChildren = node.children.length > 0}
		{@const expanded = expandedGroups.has(node.path)}
		<div
			class="group flex items-center rounded-lg transition-colors
      {active ? 'bg-accent/10' : 'hover:bg-overlay-light'}"
		>
			{#if hasChildren && node.fullTag}
				<button
					onclick={() => toggleGroup(node.path)}
					class="flex shrink-0 items-center justify-center w-5 h-5 rounded transition-transform {expanded ? '' : '-rotate-90'}"
					style="margin-left:{4 + depth * 14}px"
				>
					<Icon
						src={FiChevronRight}
						size="13"
						className={active ? "text-accent" : "text-text-muted"}
					/>
				</button>
				<button
					onclick={(e) => ontoggle(node.fullTag!, e.metaKey || e.ctrlKey)}
					class="flex flex-1 items-center gap-2.5 py-1.5 text-[13px] transition-colors
            {active ? 'text-accent' : 'text-text-secondary hover:text-text-primary'}"
				>
					{#if tc}
						<span
							class="h-2.5 w-2.5 shrink-0 rounded-full"
							style="background:{tc}"
						></span>
					{/if}
					<span>{node.label}</span>
				</button>
			{:else}
				<button
					onclick={(e) => {
						if (hasChildren) toggleGroup(node.path);
						else if (node.fullTag)
							ontoggle(node.fullTag, e.metaKey || e.ctrlKey);
					}}
					class="flex flex-1 items-center gap-2.5 px-2.5 py-1.5 text-[13px] transition-colors
            {active ? 'text-accent' : 'text-text-secondary hover:text-text-primary'}"
					style="padding-left:{10 + depth * 14}px"
				>
					{#if tc}
						<span
							class="h-2.5 w-2.5 shrink-0 rounded-full"
							style="background:{tc}"
						></span>
					{:else if hasChildren}
						<Icon
							src={FiChevronRight}
							size="13"
							className="shrink-0 transition-transform {expanded
								? 'rotate-90'
								: ''} {active ? 'text-accent' : 'text-text-muted'}"
						/>
					{:else}
						<Icon
							src={FiTag}
							size="13"
							className="shrink-0 {active ? 'text-accent' : 'text-text-muted'}"
						/>
					{/if}
					<span>{node.label}</span>
				</button>
			{/if}
			{#if node.fullTag}
				<button
					onclick={(e) => onopencolorpicker(e, node.fullTag!)}
					class="mr-1 flex h-5 w-5 items-center justify-center rounded opacity-0 transition-opacity group-hover:opacity-100 hover:bg-overlay-light"
					title="Set color"
				>
					<span
						class="h-2 w-2 rounded-full {tc
							? ''
							: 'border border-text-muted/40'}"
						style={tc ? `background:${tc}` : ""}
					></span>
				</button>
			{/if}
		</div>
		{#if hasChildren && expanded}
			{@render renderNodes(node.children, depth + 1)}
		{/if}
	{/each}
{/snippet}

{@render renderNodes(tree, 0)}

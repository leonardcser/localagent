<script lang="ts">
import { Icon } from "svelte-icons-pack";
import { FiX } from "svelte-icons-pack/fi";
import { tagColorStore } from "$lib/stores/tagColor.svelte";

let {
  tag,
  position,
  onclose,
}: {
  tag: string;
  position: { x: number; y: number };
  onclose: () => void;
} = $props();

const tc = $derived(tagColorStore.get(tag));
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
	data-color-picker
	class="fixed z-[100] grid grid-cols-5 gap-1.5 rounded-lg border border-border bg-bg-secondary p-2 shadow-elevated"
	style="left:{position.x}px;top:{position.y}px"
	onclick={(e) => e.stopPropagation()}
>
	{#each tagColorStore.palette as color}
		<button
			onclick={() => {
				tagColorStore.set(tag, color);
				onclose();
			}}
			class="h-5 w-5 rounded-full border-2 transition-transform hover:scale-110 {tc === color ? 'border-text-primary' : 'border-transparent'}"
			style="background:{color}"
			title={color}
		></button>
	{/each}
	{#if tc}
		<button
			onclick={() => {
				tagColorStore.remove(tag);
				onclose();
			}}
			class="flex h-5 w-5 items-center justify-center rounded-full border border-border text-text-muted hover:border-border-light hover:text-text-secondary"
			title="Remove color"
		>
			<Icon src={FiX} size="10" />
		</button>
	{/if}
</div>

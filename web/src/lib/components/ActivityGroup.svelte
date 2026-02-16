<script lang="ts">
import { fade } from "svelte/transition";
import ActivityItem from "./ActivityItem.svelte";
import type { TimelineItem } from "$lib/stores/chat.svelte";

type ActivityTimelineItem = Extract<TimelineItem, { kind: "activity" }>;

let {
	items,
}: {
	items: ActivityTimelineItem[];
} = $props();

let expanded = $state(false);

let latest = $derived(items[items.length - 1]);
let count = $derived(items.length);
</script>

<div class="py-1">
	{#if expanded}
		{#each items as item (item.id)}
			<ActivityItem
				event_type={item.event_type}
				timestamp={item.timestamp}
				message={item.message}
				detail={item.detail}
			/>
		{/each}
		{#if count > 1}
			<button
				class="flex items-baseline py-px w-full text-left cursor-pointer bg-transparent border-none font-[inherit]"
				onclick={() => (expanded = false)}
			>
				<span class="shrink-0 w-12"></span>
				<span class="text-[10px] text-text-muted">collapse</span>
			</button>
		{/if}
	{:else}
		<div class="activity-latest">
			{#key latest.id}
				<div in:fade={{ duration: 200 }}>
					<ActivityItem
						event_type={latest.event_type}
						timestamp={latest.timestamp}
						message={count > 1 ? `${latest.message} (+${count - 1} more)` : latest.message}
						detail={latest.detail}
						onclick={count > 1 ? () => (expanded = true) : undefined}
					/>
				</div>
			{/key}
		</div>
	{/if}
</div>

<style>
	.activity-latest {
		display: grid;
	}
	.activity-latest > :global(div) {
		grid-area: 1 / 1;
	}
</style>

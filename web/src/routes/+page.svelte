<script lang="ts">
import { onMount, onDestroy } from "svelte";
import {
	chat,
	type MessageTimelineItem,
	type ActivityTimelineItem,
} from "$lib/stores/chat.svelte";
import ChatMessage from "$lib/components/ChatMessage.svelte";
import ActivityGroup from "$lib/components/ActivityGroup.svelte";
import LoadingBubble from "$lib/components/LoadingBubble.svelte";
import InputArea from "$lib/components/InputArea.svelte";
import DropOverlay from "$lib/components/DropOverlay.svelte";
import { Icon } from "svelte-icons-pack";
import { FiChevronDown } from "svelte-icons-pack/fi";

type GroupedItem =
	| MessageTimelineItem
	| { kind: "activity-group"; id: number; items: ActivityTimelineItem[] };

let groups = $derived.by(() => {
	const result: GroupedItem[] = [];
	for (const item of chat.timeline) {
		if (item.kind === "message") {
			result.push(item as MessageTimelineItem);
		} else {
			const last = result[result.length - 1];
			if (last?.kind === "activity-group") {
				last.items.push(item);
			} else {
				result.push({ kind: "activity-group", id: item.id, items: [item] });
			}
		}
	}
	return result;
});

let messagesEl: HTMLDivElement;
let isAtBottom = $state(true);
let programmaticScroll = false;
let resizeObserver: ResizeObserver | null = null;

function isScrolledToBottom(): boolean {
	if (!messagesEl) return true;
	return (
		messagesEl.scrollHeight - messagesEl.scrollTop - messagesEl.clientHeight <
		40
	);
}

function snapToBottom() {
	if (messagesEl) {
		programmaticScroll = true;
		messagesEl.scrollTop = messagesEl.scrollHeight;
		requestAnimationFrame(() => {
			programmaticScroll = false;
			isAtBottom = true;
		});
	}
}

function scrollToBottom() {
	if (messagesEl) {
		programmaticScroll = true;
		messagesEl.scrollTo({ top: messagesEl.scrollHeight, behavior: "smooth" });
		setTimeout(() => {
			programmaticScroll = false;
			isAtBottom = true;
		}, 300);
	}
}

function handleScroll() {
	if (programmaticScroll) return;
	isAtBottom = isScrolledToBottom();
}

$effect(() => {
	if (!messagesEl) return;
	resizeObserver = new ResizeObserver(() => {
		if (isAtBottom) snapToBottom();
	});
	for (const child of messagesEl.children) {
		resizeObserver.observe(child);
	}
	const mutObserver = new MutationObserver((mutations) => {
		for (const m of mutations) {
			for (const node of m.addedNodes) {
				if (node instanceof Element) resizeObserver!.observe(node);
			}
		}
		if (isAtBottom) snapToBottom();
	});
	mutObserver.observe(messagesEl, { childList: true });
	return () => {
		resizeObserver!.disconnect();
		mutObserver.disconnect();
	};
});

let dragCounter = 0;

function handleDragEnter(e: DragEvent) {
	e.preventDefault();
	dragCounter++;
	if (dragCounter === 1 && e.dataTransfer?.types.includes("Files")) {
		chat.dragging = true;
	}
}

function handleDragOver(e: DragEvent) {
	e.preventDefault();
}

function handleDragLeave(e: DragEvent) {
	e.preventDefault();
	dragCounter--;
	if (dragCounter === 0) {
		chat.dragging = false;
	}
}

function handleDrop(e: DragEvent) {
	e.preventDefault();
	dragCounter = 0;
	if (e.dataTransfer?.files) {
		chat.handleDrop(e.dataTransfer.files);
	}
}

function handleVisibility() {
	chat.reportVisibility();
}

function handleFocus() {
	chat.reportVisibility();
}

function handleBlur() {
	chat.reportVisibility();
}

onMount(() => {
	chat.init();
	document.addEventListener("visibilitychange", handleVisibility);
	window.addEventListener("focus", handleFocus);
	window.addEventListener("blur", handleBlur);
	document.addEventListener("dragenter", handleDragEnter);
	document.addEventListener("dragover", handleDragOver);
	document.addEventListener("dragleave", handleDragLeave);
	document.addEventListener("drop", handleDrop);
});

onDestroy(() => {
	chat.destroy();
	if (typeof document !== "undefined") {
		document.removeEventListener("visibilitychange", handleVisibility);
		window.removeEventListener("focus", handleFocus);
		window.removeEventListener("blur", handleBlur);
		document.removeEventListener("dragenter", handleDragEnter);
		document.removeEventListener("dragover", handleDragOver);
		document.removeEventListener("dragleave", handleDragLeave);
		document.removeEventListener("drop", handleDrop);
	}
});
</script>

<div class="relative mx-auto flex h-full w-full max-w-3xl flex-col">
	<div
		bind:this={messagesEl}
		onscroll={handleScroll}
		class="messages-scroll flex flex-1 flex-col gap-3 overflow-y-auto px-4 py-4 [-webkit-overflow-scrolling:touch]"
	>
		{#each groups as group (group.id)}
			{#if group.kind === "message"}
				<ChatMessage role={group.role} content={group.content} timestamp={group.timestamp} media={group.media} queued={group.queued} />
			{:else}
				<ActivityGroup
				items={group.items}
				expanded={chat.isGroupExpanded(group.items[0].timestamp)}
				onToggle={() => chat.toggleGroupExpanded(group.items[0].timestamp)}
				isItemExpanded={chat.isGroupExpanded}
				toggleItemExpanded={chat.toggleGroupExpanded}
			/>
			{/if}
		{/each}
		{#if chat.loading}
			<LoadingBubble />
		{/if}
		{#if chat.timeline.length === 0 && !chat.loading}
			<div class="flex h-full flex-col items-center justify-center gap-2">
				<span class="text-[13px] text-text-muted">Send a message to start.</span>
			</div>
		{/if}
	</div>

	{#if !isAtBottom}
		<button
			class="absolute bottom-20 inset-x-0 mx-auto z-10 flex items-center justify-center w-9 h-9 rounded-full border border-border-light bg-bg-secondary text-text-secondary cursor-pointer shadow-elevated transition-[background,color] duration-150 hover:bg-surface hover:text-text-primary"
			onclick={scrollToBottom}
			title="Scroll to bottom"
		>
			<Icon src={FiChevronDown} size="18" />
		</button>
	{/if}

	<InputArea />
	{#if chat.dragging}
		<DropOverlay />
	{/if}
</div>

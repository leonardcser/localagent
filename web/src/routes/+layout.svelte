<script lang="ts">
import "../app.css";
import { onMount } from "svelte";
import { page } from "$app/state";
import { Icon } from "svelte-icons-pack";
import {
	FiMessageCircle,
	FiImage,
	FiSidebar,
	FiBell,
	FiBellOff,
} from "svelte-icons-pack/fi";
import { push } from "$lib/stores/push.svelte";

let { children } = $props();

let collapsed = $state(true);

onMount(() => {
	function preventZoom(e: TouchEvent) {
		if (e.touches.length > 1) e.preventDefault();
	}
	function preventGesture(e: Event) {
		e.preventDefault();
	}
	document.addEventListener("touchmove", preventZoom, { passive: false });
	document.addEventListener("gesturestart", preventGesture, { passive: false });
	document.addEventListener("gesturechange", preventGesture, {
		passive: false,
	});
	return () => {
		document.removeEventListener("touchmove", preventZoom);
		document.removeEventListener("gesturestart", preventGesture);
		document.removeEventListener("gesturechange", preventGesture);
	};
});

const navItems = [
	{ href: "/", icon: FiMessageCircle, label: "Chat" },
	{ href: "/images", icon: FiImage, label: "Images" },
];

function isActive(href: string): boolean {
	if (href === "/") return page.url.pathname === "/";
	return page.url.pathname.startsWith(href);
}
</script>

<div class="fixed inset-0 flex">
	<nav
		class="flex flex-col border-r border-border bg-bg-secondary transition-[width] duration-150"
		style:width={collapsed ? "48px" : "160px"}
	>
		<div class="flex flex-col gap-1 p-1.5 pt-[max(env(safe-area-inset-top,0px),6px)]">
			{#each navItems as item}
				<a
					href={item.href}
					class="flex items-center rounded-md py-2 text-[13px] transition-colors duration-100 {collapsed
						? 'justify-center'
						: 'gap-2.5 px-2.5'} {isActive(item.href)
						? 'bg-overlay-medium text-text-primary'
						: 'text-text-secondary hover:bg-overlay-light hover:text-text-primary'}"
					title={collapsed ? item.label : undefined}
				>
					<Icon src={item.icon} size="18" className="shrink-0" />
					{#if !collapsed}
						<span class="truncate">{item.label}</span>
					{/if}
				</a>
			{/each}
		</div>
		<div class="mt-auto flex flex-col gap-1 p-1.5 pb-[max(env(safe-area-inset-bottom,0px),6px)]">
			{#if push.supported}
				<button
					onclick={() => push.subscribe()}
					class="flex w-full items-center rounded-md py-2 text-text-muted transition-colors duration-100 hover:bg-overlay-light hover:text-text-secondary {collapsed
						? 'justify-center'
						: 'gap-2.5 px-2.5'}"
					title={collapsed
						? push.permission === "granted"
							? "Notifications enabled"
							: "Enable notifications"
						: undefined}
				>
					<Icon
						src={push.permission === "granted" ? FiBell : FiBellOff}
						size="16"
						className="shrink-0 {push.permission === 'granted' ? 'text-green-400' : ''}"
					/>
					{#if !collapsed}
						<span class="truncate text-[12px]">
							{push.permission === "granted"
								? "Notifications on"
								: "Enable notifications"}
						</span>
					{/if}
				</button>
			{/if}
			<button
				onclick={() => (collapsed = !collapsed)}
				class="flex w-full items-center rounded-md py-2 text-text-muted transition-colors duration-100 hover:bg-overlay-light hover:text-text-secondary {collapsed
					? 'justify-center'
					: 'gap-2.5 px-2.5'}"
				title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
			>
				<Icon src={FiSidebar} size="16" className="shrink-0" />
				{#if !collapsed}
					<span class="truncate text-[12px]">Collapse</span>
				{/if}
			</button>
		</div>
	</nav>
	<main class="flex-1 overflow-hidden">
		{@render children()}
	</main>
</div>

<script lang="ts">
import "../app.css";
import { onMount } from "svelte";
import { page } from "$app/state";
import { Icon } from "svelte-icons-pack";
import {
	FiMessageCircle,
	FiCheckSquare,
	FiImage,
	FiMenu,
	FiBell,
	FiBellOff,
} from "svelte-icons-pack/fi";
import { push } from "$lib/stores/push.svelte";

let { children } = $props();

let menuOpen = $state(false);

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
	{ href: "/tasks", icon: FiCheckSquare, label: "Tasks" },
	{ href: "/images", icon: FiImage, label: "Images" },
];

function isActive(href: string): boolean {
	if (href === "/") return page.url.pathname === "/";
	return page.url.pathname.startsWith(href);
}

let pageTitle = $derived(
	navItems.find((item) => isActive(item.href))?.label ?? "Chat",
);
</script>

<div class="fixed inset-0 flex flex-col md:flex-row">
	<!-- Mobile top bar -->
	<header
		class="z-30 flex shrink-0 items-center border-b border-border bg-chrome backdrop-blur-md md:hidden"
		style="padding-top: env(safe-area-inset-top, 0px)"
	>
		<button
			class="flex h-10 w-10 items-center justify-center text-text-secondary"
			onclick={() => (menuOpen = !menuOpen)}
		>
			<Icon src={FiMenu} size="20" />
		</button>
		<span class="text-[13px] font-medium text-text-primary">{pageTitle}</span>
	</header>

	<!-- Mobile backdrop -->
	{#if menuOpen}
		<button
			class="fixed inset-0 z-10 bg-black/40 backdrop-blur-sm transition-opacity md:hidden"
			onclick={() => (menuOpen = false)}
			tabindex="-1"
			aria-label="Close menu"
		></button>
	{/if}

	<!-- Sidebar nav -->
	<nav
		class="fixed left-0 top-0 z-20 flex h-full w-48 flex-col border-r border-border bg-bg-secondary transition-transform duration-200
			{menuOpen ? 'translate-x-0' : '-translate-x-full'}
			md:relative md:w-12 md:translate-x-0"
	>
		<div class="flex flex-col gap-1 p-1.5 pt-[max(env(safe-area-inset-top,0px),6px)]">
			{#each navItems as item}
				<a
					href={item.href}
					onclick={() => (menuOpen = false)}
					class="flex items-center rounded-md py-2 text-[13px] transition-colors duration-100
						gap-2.5 px-2.5 md:justify-center md:gap-0 md:px-0
						{isActive(item.href)
						? 'bg-overlay-medium text-text-primary'
						: 'text-text-secondary hover:bg-overlay-light hover:text-text-primary'}"
					title={item.label}
				>
					<Icon src={item.icon} size="18" className="shrink-0" />
					<span class="truncate md:hidden">{item.label}</span>
				</a>
			{/each}
		</div>
		<div class="mt-auto flex flex-col gap-1 p-1.5 pb-[max(env(safe-area-inset-bottom,0px),6px)]">
			{#if push.supported}
				<button
					onclick={() => push.subscribe()}
					class="flex w-full items-center rounded-md py-2 text-text-muted transition-colors duration-100 hover:bg-overlay-light hover:text-text-secondary
						gap-2.5 px-2.5 md:justify-center md:gap-0 md:px-0"
					title={push.permission === "granted"
						? "Notifications enabled"
						: "Enable notifications"}
				>
					<Icon
						src={push.permission === "granted" ? FiBell : FiBellOff}
						size="16"
						className="shrink-0 {push.permission === 'granted' ? 'text-green-400' : ''}"
					/>
					<span class="truncate text-[12px] md:hidden">
						{push.permission === "granted"
							? "Notifications on"
							: "Enable notifications"}
					</span>
				</button>
			{/if}
		</div>
	</nav>
	<main class="flex-1 overflow-hidden md:pt-[env(safe-area-inset-top,0px)]">
		{@render children()}
	</main>
</div>

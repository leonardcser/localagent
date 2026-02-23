<script lang="ts">
import { renderMarkdown, COPY_SVG, CHECK_SVG } from "$lib/markdown";
import { cn, filename, isAudio, mediaUrl, formatTimestamp } from "$lib/utils";
import { Icon } from "svelte-icons-pack";
import { FiFile } from "svelte-icons-pack/fi";

let {
	role,
	content,
	timestamp,
	media,
	queued,
}: {
	role: string;
	content: string;
	timestamp: string;
	media?: string[];
	queued?: boolean;
} = $props();

let html = $state("");

$effect(() => {
	renderMarkdown(content).then((result) => {
		html = result;
	});
});

function handleClick(e: MouseEvent) {
	const btn = (e.target as HTMLElement).closest(
		".copy-btn",
	) as HTMLButtonElement | null;
	if (!btn?.dataset.code) return;
	navigator.clipboard.writeText(btn.dataset.code);
	btn.innerHTML = CHECK_SVG;
	setTimeout(() => {
		btn.innerHTML = COPY_SVG;
	}, 1500);
}
</script>

{#if role === "user"}
	<div class="flex flex-col items-end self-end min-w-0 max-w-[85%] sm:max-w-[75%]">
		<div class="user-msg max-w-full overflow-hidden rounded-2xl rounded-br-md bg-user-bubble px-3.5 py-2.5 text-[14px] leading-relaxed text-user-bubble-text">
			{#if media && media.length > 0}
				<div class={cn("flex flex-col gap-1.5", content && "mb-1.5")}>
					{#each media as path (path)}
						{#if isAudio(path)}
							<audio class="audio-player" controls preload="metadata" src={mediaUrl(path)}>
								<track kind="captions" />
							</audio>
						{:else}
							<span class="inline-flex items-center gap-1.5 rounded-md bg-white/15 px-2 py-1 text-[11px] text-user-bubble-text/80">
								<Icon src={FiFile} size="12" />
								<span class="max-w-30 truncate" title={filename(path)}>{filename(path)}</span>
							</span>
						{/if}
					{/each}
				</div>
			{/if}
			{#if content}
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="msg-content" onclick={handleClick} onkeydown={() => {}}>{@html html}</div>
			{/if}
		</div>
		<span class="mt-1 text-[10px] font-mono text-text-muted">
			{#if queued}<span class="text-text-muted/60">queued &middot; </span>{/if}{formatTimestamp(timestamp)}
		</span>
	</div>
{:else}
	<div class="flex flex-col items-start self-start min-w-0 max-w-[95%] sm:max-w-[85%]">
		<div class="assistant-msg max-w-full overflow-hidden rounded-2xl rounded-bl-md bg-bg-secondary px-3.5 py-2.5 text-[14px] leading-relaxed text-text-primary">
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="msg-content" onclick={handleClick} onkeydown={() => {}}>{@html html}</div>
		</div>
		<span class="mt-1 text-[10px] font-mono text-text-muted">{formatTimestamp(timestamp)}</span>
	</div>
{/if}

<style>
	.audio-player {
		width: 100%;
		min-width: 200px;
		max-width: 300px;
		height: 36px;
		border-radius: 0.5rem;
		opacity: 0.9;
	}

	.msg-content {
		min-width: 0;
		overflow: hidden;
	}

	.msg-content :global(p) {
		margin: 0 0 0.5em;
	}

	.msg-content :global(p:last-child) {
		margin-bottom: 0;
	}

	.msg-content :global(.code-block-wrapper) {
		border-radius: 0.5rem;
		overflow: hidden;
		margin: 0.5rem 0;
		border: 1px solid var(--color-border-light);
	}

	.msg-content :global(.code-header) {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.1rem 0.5rem;
		background: var(--color-overlay-subtle);
		border-bottom: 1px solid var(--color-border-light);
		font-size: 0.75rem;
	}

	.msg-content :global(.code-lang) {
		color: var(--color-text-muted);
		font-family: var(--font-mono);
		font-size: 0.7rem;
	}

	.msg-content :global(.copy-btn) {
		display: flex;
		align-items: center;
		gap: 0.35rem;
		margin-left: auto;
		padding: 0.15rem 0.4rem;
		color: var(--color-text-muted);
		background: transparent;
		border: none;
		border-radius: 0.25rem;
		cursor: pointer;
		font-family: inherit;
		font-size: 0.7rem;
		transition: color 0.15s, background 0.15s;
	}

	.msg-content :global(.copy-btn:hover) {
		color: var(--color-text-primary);
		background: var(--color-overlay-light);
	}

	.msg-content :global(pre) {
		border-radius: 0;
		overflow-x: auto;
		margin: 0;
		font-size: 0.8125rem;
	}

	.msg-content :global(.shiki) {
		padding: 0.5rem 0.75rem;
		border-radius: 0;
		overflow-x: auto;
		margin: 0;
		font-size: 0.8125rem;
		line-height: 1.5;
	}

	.msg-content :global(code) {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
	}

	.msg-content :global(code:not(pre code)) {
		background: var(--color-border);
		padding: 0.15rem 0.35rem;
		border-radius: 0.25rem;
		font-size: 0.85em;
	}

	.user-msg .msg-content :global(code:not(pre code)) {
		background: var(--color-overlay-medium);
	}

	.msg-content :global(a) {
		color: var(--color-accent);
		text-decoration: none;
	}

	.msg-content :global(a:hover) {
		text-decoration: underline;
	}

	.user-msg .msg-content :global(a) {
		color: inherit;
		text-decoration: underline;
		text-decoration-color: var(--color-overlay-strong);
	}

	.msg-content :global(ul) {
		padding-left: 1.5em;
		margin: 0.25em 0;
		list-style: disc;
	}

	.msg-content :global(ol) {
		padding-left: 1.5em;
		margin: 0.25em 0;
		list-style: decimal;
	}

	.msg-content :global(li) {
		margin: 0.15em 0;
	}

	.msg-content :global(blockquote) {
		border-left: 3px solid var(--color-border-light);
		padding-left: 0.75rem;
		margin: 0.5em 0;
		color: var(--color-text-secondary);
	}

	.msg-content :global(h1),
	.msg-content :global(h2),
	.msg-content :global(h3),
	.msg-content :global(h4) {
		margin: 0.75em 0 0.35em;
		font-weight: 600;
		line-height: 1.3;
	}

	.msg-content :global(h1) {
		font-size: 1.25em;
	}

	.msg-content :global(h2) {
		font-size: 1.125em;
	}

	.msg-content :global(h3) {
		font-size: 1em;
	}

	.msg-content :global(hr) {
		border: none;
		border-top: 1px solid var(--color-border);
		margin: 0.75em 0;
	}

	.msg-content :global(.table-wrapper) {
		overflow-x: auto;
		margin: 0.5em 0;
		border: 1px solid var(--color-border-light);
		border-radius: 0.5rem;
	}

	.msg-content :global(table) {
		border-collapse: separate;
		border-spacing: 0;
		font-size: 0.875em;
		min-width: 100%;
		border-radius: 0.5rem;
		overflow: hidden;
	}

	.msg-content :global(th),
	.msg-content :global(td) {
		border-bottom: 1px solid var(--color-border-light);
		border-right: 1px solid var(--color-border-light);
		padding: 0.4em 0.6em;
		text-align: left;
	}

	.msg-content :global(th:last-child),
	.msg-content :global(td:last-child) {
		border-right: none;
	}

	.msg-content :global(tr:last-child td) {
		border-bottom: none;
	}

	.msg-content :global(th) {
		background: var(--color-surface);
		font-weight: 600;
	}
</style>

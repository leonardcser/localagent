<script lang="ts">
import { cn } from "$lib/utils";
import { chat } from "$lib/stores/chat.svelte";
import MediaPreview from "./MediaPreview.svelte";
import AudioWaveform from "./AudioWaveform.svelte";
import { Icon } from "svelte-icons-pack";
import { FiPaperclip, FiMic, FiArrowUp } from "svelte-icons-pack/fi";

let textarea = $state<HTMLTextAreaElement>();
let fileInput = $state<HTMLInputElement>();

function handleKeydown(e: KeyboardEvent) {
	if (e.key === "Enter" && !e.shiftKey) {
		e.preventDefault();
		chat.send();
	}
}

function autoGrow() {
	if (!textarea) return;
	textarea.style.height = "auto";
	textarea.style.height = `${Math.min(textarea.scrollHeight, 200)}px`;
}

function handleAttach() {
	fileInput?.click();
}

function handleFileChange(e: Event) {
	const target = e.target as HTMLInputElement;
	if (target.files) {
		chat.attachFiles(target.files);
		target.value = "";
	}
}

$effect(() => {
	chat.input;
	if (textarea) {
		textarea.style.height = "auto";
		textarea.style.height = `${Math.min(textarea.scrollHeight, 200)}px`;
	}
});
</script>

<div class="input-chrome shrink-0 px-3 py-2.5 pb-[calc(0.625rem+env(safe-area-inset-bottom,0px))] sm:px-4">
	{#if chat.pendingMedia.length > 0}
		<MediaPreview media={chat.pendingMedia} onRemove={(i) => chat.removeMedia(i)} />
	{/if}
	<form
		class="flex items-end gap-1.5"
		onsubmit={(e) => {
			e.preventDefault();
			chat.send();
		}}
	>
		{#if chat.recording || chat.transcribing}
			<div class="flex h-10.5 flex-1 items-center rounded-2xl bg-bg-tertiary ring-1 ring-border-light overflow-hidden px-3">
				<AudioWaveform mode={chat.recording ? "recording" : "transcribing"} stream={chat.mediaStream} />
			</div>
		{:else}
			<div class="flex min-h-10.5 flex-1 items-end rounded-2xl bg-bg-tertiary ring-1 ring-border-light transition-all duration-150 focus-within:ring-text-muted">
				<textarea
					bind:this={textarea}
					bind:value={chat.input}
					oninput={autoGrow}
					onkeydown={handleKeydown}
					placeholder="Message..."
					rows="1"
					class="max-h-50 min-h-10.5 flex-1 resize-none overflow-y-auto bg-transparent px-4 py-2.5 text-[14px] leading-normal text-text-primary outline-none placeholder:text-text-muted"
				></textarea>
				<button
					type="button"
					class="mb-1 mr-1 flex size-8.5 shrink-0 cursor-pointer items-center justify-center rounded-full border-none bg-transparent text-text-muted transition-[color,background] duration-150 hover:bg-border hover:text-text-secondary"
					onclick={handleAttach}
					title="Attach file"
				>
					<Icon src={FiPaperclip} size="18" />
				</button>
			</div>
		{/if}
		<input
			bind:this={fileInput}
			type="file"
			multiple
			onchange={handleFileChange}
			hidden
		/>
		<button
			type="button"
			class={cn(
				"flex size-10.5 shrink-0 cursor-pointer items-center justify-center rounded-full border-none bg-transparent text-text-muted transition-[color,background] duration-150 hover:bg-border hover:text-text-primary disabled:cursor-not-allowed",
				chat.transcribing && "animate-pulse bg-surface! text-text-secondary! cursor-wait transition-none!",
				!chat.transcribing && chat.recording && "recording",
			)}
			onclick={() => chat.toggleRecording()}
			disabled={chat.loading || chat.transcribing}
			title={chat.transcribing ? "Transcribing..." : "Voice"}
		>
			{#if chat.transcribing}
				<Icon src={FiMic} size="18" />
			{:else if chat.recording}
				<span class="block size-3.5 rounded-xs bg-current"></span>
			{:else}
				<Icon src={FiMic} size="18" />
			{/if}
		</button>
		<button
			type="button"
			class={cn(
				"send-btn flex size-10.5 shrink-0 cursor-pointer items-center justify-center rounded-full border-none bg-transparent text-text-muted opacity-40 pointer-events-none transition-[color,background,opacity] duration-150",
				(chat.input.trim().length > 0 || chat.pendingMedia.length > 0 || chat.recording) && !chat.transcribing && "has-input",
			)}
			onclick={() => {
				if (chat.recording) {
					chat.recordAndSend();
				} else {
					chat.send();
				}
			}}
			title="Send"
		>
			<Icon src={FiArrowUp} size="18" />
		</button>
	</form>
</div>

<style>
	.input-chrome {
		background: var(--color-chrome);
		backdrop-filter: blur(20px);
		-webkit-backdrop-filter: blur(20px);
		border-top: 1px solid var(--color-border);
	}

	.recording {
		background: var(--color-danger) !important;
		color: white !important;
		animation: pulse-ring 1.5s infinite;
	}

	.send-btn.has-input {
		opacity: 1;
		pointer-events: auto;
		background: var(--color-text-primary);
		color: var(--color-bg);
	}

	.send-btn.has-input:hover {
		background: var(--color-text-secondary);
	}

	@keyframes pulse-ring {
		0% {
			box-shadow: 0 0 0 0 color-mix(in srgb, var(--color-danger) 40%, transparent);
		}
		70% {
			box-shadow: 0 0 0 8px color-mix(in srgb, var(--color-danger) 0%, transparent);
		}
		100% {
			box-shadow: 0 0 0 0 color-mix(in srgb, var(--color-danger) 0%, transparent);
		}
	}
</style>

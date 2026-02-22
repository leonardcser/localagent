<script lang="ts">
import { cn, formatTimestamp } from "$lib/utils";

let {
	event_type,
	timestamp,
	message,
	detail,
	onclick,
	expanded = false,
	onToggleExpand,
}: {
	event_type: string;
	timestamp: string;
	message: string;
	detail?: Record<string, unknown>;
	onclick?: () => void;
	expanded?: boolean;
	onToggleExpand?: () => void;
} = $props();

function isToolError(): boolean {
	return event_type === "tool_result" && detail?.status === "error";
}

function labelColor(t: string): string {
	if (t === "llm_error" || isToolError()) return "text-error";
	if (t.startsWith("llm_")) return "text-accent";
	if (t.startsWith("tool_")) return "text-warning";
	if (t === "complete") return "text-success";
	return "text-text-muted";
}

function label(t: string): string {
	if (isToolError()) return "ERROR";
	const labels: Record<string, string> = {
		processing_start: "START",
		llm_request: "LLM",
		llm_response: "LLM",
		llm_error: "ERROR",
		tool_call: "TOOL",
		tool_result: "RESULT",
		complete: "DONE",
	};
	return labels[t] ?? t.toUpperCase();
}
</script>

<button
	class="flex w-full items-baseline py-px text-left cursor-pointer bg-transparent border-none font-[inherit]"
	onclick={() => { if (onclick) { onclick(); } else if (detail && onToggleExpand) { onToggleExpand(); } }}
>
	<span class={cn("text-[10px] font-bold font-mono tracking-wide shrink-0 w-12", labelColor(event_type))}>{label(event_type)}</span>
	<span class={cn("text-[11px] leading-4.5 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap", isToolError() ? "text-error/80" : "text-text-muted")} title={message}>
		{message}
	</span>
	<span class="ml-auto pl-2 text-[10px] text-text-muted/50 font-mono shrink-0">{formatTimestamp(timestamp)}</span>
</button>
{#if expanded && detail}
	<pre class="ml-12 mb-0.5 px-2 py-1 text-[10px] font-mono text-text-muted bg-bg-tertiary rounded overflow-x-auto whitespace-pre-wrap break-all">{JSON.stringify(detail, null, 2)}</pre>
{/if}

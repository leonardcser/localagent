<script lang="ts">
import { Icon } from "svelte-icons-pack";
import { FiFile, FiX } from "svelte-icons-pack/fi";

let {
	media,
	onRemove,
}: { media: string[]; onRemove: (index: number) => void } = $props();

function filename(path: string): string {
	return path.split("/").pop() || path;
}
</script>

<div class="mb-2 flex flex-wrap gap-1.5">
	{#each media as path, i}
		<span class="inline-flex items-center gap-1.5 rounded-lg bg-bg-tertiary px-2.5 py-1.5 text-xs text-text-secondary ring-1 ring-border">
			<Icon src={FiFile} size="13" />
			<span class="max-w-30 truncate">{filename(path)}</span>
			<button
				type="button"
				class="ml-0.5 flex cursor-pointer items-center border-none bg-transparent p-0 text-text-muted transition-colors hover:text-danger"
				onclick={() => onRemove(i)}
				title="Remove file"
				aria-label="Remove file"
			>
				<Icon src={FiX} size="13" />
			</button>
		</span>
	{/each}
</div>

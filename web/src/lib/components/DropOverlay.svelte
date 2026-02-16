<script lang="ts">
import { Icon } from "svelte-icons-pack";
import { FiUpload } from "svelte-icons-pack/fi";
</script>

<div
	class="pointer-events-none fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
>
	<div
		class="flex flex-col items-center gap-4 rounded-3xl border-2 border-dashed border-accent/50 bg-bg-secondary/80 px-20 py-14"
	>
		<span class="text-accent/60">
			<Icon src={FiUpload} size="48" />
		</span>
		<span class="text-base font-medium text-text-secondary">
			Drop files to attach
		</span>
	</div>
</div>

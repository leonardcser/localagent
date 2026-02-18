<script lang="ts">
import { onMount, onDestroy } from "svelte";
import { imageStore } from "$lib/stores/image.svelte";
import { imageResultUrl, imageSourceUrl } from "$lib/api";
import { Icon } from "svelte-icons-pack";
import {
	FiLoader,
	FiAlertCircle,
	FiZap,
	FiDownload,
	FiTrash2,
	FiX,
	FiEdit2,
	FiUpload,
	FiArrowUp,
} from "svelte-icons-pack/fi";

let lightboxJob = $state<{ jobId: string; index: number } | null>(null);
let upscaleMenu = $state<{ jobId: string; index: number } | null>(null);
let lightboxUrl = $derived(
	lightboxJob ? imageResultUrl(lightboxJob.jobId, lightboxJob.index) : null,
);
let dragOver = $state(false);
let fileInput = $state<HTMLInputElement>(null!);

function openLightbox(jobId: string, index: number) {
	lightboxJob = { jobId, index };
}

function closeLightbox() {
	lightboxJob = null;
}

function handleKeydown(e: KeyboardEvent) {
	if (e.key === "Escape") {
		closeLightbox();
		upscaleMenu = null;
	}
	if (!lightboxJob) return;
	const job = imageStore.jobs.find((j) => j.id === lightboxJob!.jobId);
	if (!job) return;
	if (e.key === "ArrowRight" || e.key === "l") {
		if (lightboxJob.index < job.image_count - 1) {
			lightboxJob = { jobId: lightboxJob.jobId, index: lightboxJob.index + 1 };
		}
	} else if (e.key === "ArrowLeft" || e.key === "h") {
		if (lightboxJob.index > 0) {
			lightboxJob = { jobId: lightboxJob.jobId, index: lightboxJob.index - 1 };
		}
	}
}

function handleSubmit(e: SubmitEvent) {
	e.preventDefault();
	imageStore.generate();
}

function handleDrop(e: DragEvent) {
	e.preventDefault();
	dragOver = false;
	const files = Array.from(e.dataTransfer?.files ?? []).filter((f) =>
		f.type.startsWith("image/"),
	);
	if (files.length > 0) imageStore.addSourceImages(files);
}

function handleDragOver(e: DragEvent) {
	e.preventDefault();
	dragOver = true;
}

function handleDragLeave() {
	dragOver = false;
}

function handleFileSelect(e: Event) {
	const input = e.target as HTMLInputElement;
	const files = Array.from(input.files ?? []);
	if (files.length > 0) imageStore.addSourceImages(files);
	input.value = "";
}

onMount(() => {
	imageStore.init();
	document.addEventListener("keydown", handleKeydown);
});

onDestroy(() => {
	imageStore.destroy();
	if (typeof document !== "undefined") {
		document.removeEventListener("keydown", handleKeydown);
	}
});

let reversedJobs = $derived([...imageStore.jobs].reverse());
</script>

<div class="flex h-full">
	<!-- Controls Panel -->
	<div class="flex w-72 shrink-0 flex-col border-r border-border bg-bg-secondary">
		<div class="p-4">
			<h2 class="text-[13px] font-medium text-text-primary">{imageStore.isUpscaleModel ? "Upscale" : imageStore.isEditModel ? "Edit" : "Generate"}</h2>
		</div>

		<form onsubmit={handleSubmit} class="flex flex-1 flex-col gap-3 overflow-y-auto px-4 pb-4">
			<!-- Model -->
			<label class="flex flex-col gap-1">
				<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Model</span>
				<select
					bind:value={imageStore.selectedModel}
					class="rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary outline-none focus:border-accent"
				>
					{#if imageStore.modelGroups.generate.length > 0}
						<optgroup label="Generate">
							{#each imageStore.modelGroups.generate as model}
								<option value={model}>{model}</option>
							{/each}
						</optgroup>
					{/if}
					{#if imageStore.modelGroups.edit.length > 0}
						<optgroup label="Edit">
							{#each imageStore.modelGroups.edit as model}
								<option value={model}>{model}</option>
							{/each}
						</optgroup>
					{/if}
					{#if imageStore.modelGroups.upscale.length > 0}
						<optgroup label="Upscale">
							{#each imageStore.modelGroups.upscale as model}
								<option value={model}>{model}</option>
							{/each}
						</optgroup>
					{/if}
					{#if imageStore.models.length === 0}
						<option value="" disabled>No models available</option>
					{/if}
				</select>
			</label>

			{#if !imageStore.isUpscaleModel}
				<!-- Prompt -->
				<label class="flex flex-col gap-1">
					<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Prompt</span>
					<textarea
						bind:value={imageStore.prompt}
						rows="3"
						placeholder={imageStore.isEditModel ? "Describe the edit you want to make..." : "Describe the image you want to create..."}
						class="resize-none rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
					></textarea>
				</label>

				<!-- Negative Prompt -->
				<label class="flex flex-col gap-1">
					<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Negative Prompt</span>
					<textarea
						bind:value={imageStore.negativePrompt}
						rows="2"
						placeholder="What to avoid..."
						class="resize-none rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
					></textarea>
				</label>
			{/if}

			{#if imageStore.isEditModel || imageStore.isUpscaleModel}
				<!-- Source Images Drop Zone -->
				<div class="flex flex-col gap-1">
					<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Source Images</span>
					<button
						type="button"
						ondrop={handleDrop}
						ondragover={handleDragOver}
						ondragleave={handleDragLeave}
						onclick={() => fileInput.click()}
						class="flex flex-col items-center justify-center gap-1.5 rounded-md border-2 border-dashed px-3 py-4 text-[12px] transition-colors duration-100 {dragOver ? 'border-accent bg-accent/10 text-accent' : 'border-border text-text-muted hover:border-border-light hover:text-text-secondary'}"
					>
						<Icon src={FiUpload} size="16" />
						<span>Drop images or click to browse</span>
					</button>
					<input
						bind:this={fileInput}
						type="file"
						accept="image/*"
						multiple
						onchange={handleFileSelect}
						class="hidden"
					/>
					{#if imageStore.sourceImages.length > 0}
						<div class="mt-1 flex flex-wrap gap-1.5">
							{#each imageStore.sourceImages as file, i}
								<div class="group relative">
									<img
										src={URL.createObjectURL(file)}
										alt="Source {i + 1}"
										class="h-14 w-14 rounded border border-border object-cover"
									/>
									<button
										type="button"
										onclick={() => imageStore.removeSourceImage(i)}
										class="absolute -right-1 -top-1 flex h-4 w-4 items-center justify-center rounded-full bg-black/70 text-white opacity-0 transition-opacity duration-100 group-hover:opacity-100"
									>
										<Icon src={FiX} size="10" />
									</button>
								</div>
							{/each}
						</div>
					{/if}
				</div>
			{:else}
				<!-- Dimensions -->
				<div class="flex gap-2">
					<label class="flex min-w-0 flex-1 flex-col gap-1">
						<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Width</span>
						<input
							type="number"
							bind:value={imageStore.width}
							min="256"
							max="2048"
							step="16"
							class="min-w-0 rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary outline-none focus:border-accent"
						/>
					</label>
					<label class="flex min-w-0 flex-1 flex-col gap-1">
						<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Height</span>
						<input
							type="number"
							bind:value={imageStore.height}
							min="256"
							max="2048"
							step="16"
							class="min-w-0 rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary outline-none focus:border-accent"
						/>
					</label>
				</div>
			{/if}

			{#if imageStore.selectedModel === "seedvr2"}
				<label class="flex flex-col gap-1">
					<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Scale</span>
					<input
						type="text"
						bind:value={imageStore.scale}
						placeholder="2"
						class="min-w-0 rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
					/>
				</label>
			{:else if !imageStore.isUpscaleModel}
				<!-- Seed + Count -->
				<div class="flex gap-2">
					<label class="flex min-w-0 flex-1 flex-col gap-1">
						<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Seed</span>
						<input
							type="text"
							bind:value={imageStore.seed}
							placeholder="Random"
							class="min-w-0 rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
						/>
					</label>
					<label class="flex w-16 shrink-0 flex-col gap-1">
						<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Count</span>
						<input
							type="number"
							bind:value={imageStore.count}
							min="1"
							max="4"
							class="min-w-0 rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary outline-none focus:border-accent"
						/>
					</label>
				</div>

				<!-- Steps + Guidance Scale -->
				<div class="flex gap-2">
					<label class="flex min-w-0 flex-1 flex-col gap-1">
						<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Steps</span>
						<input
							type="text"
							bind:value={imageStore.steps}
							placeholder="Default"
							class="min-w-0 rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
						/>
					</label>
					<label class="flex min-w-0 flex-1 flex-col gap-1">
						<span class="text-[11px] font-medium uppercase tracking-wider text-text-muted">Guidance</span>
						<input
							type="text"
							bind:value={imageStore.guidanceScale}
							placeholder="Default"
							class="min-w-0 rounded-md border border-border bg-bg-tertiary px-2.5 py-1.5 text-[13px] text-text-primary placeholder:text-text-muted outline-none focus:border-accent"
						/>
					</label>
				</div>
			{/if}

			<div class="mt-auto pt-2">
				<button
					type="submit"
					disabled={!imageStore.selectedModel || imageStore.generating || (imageStore.isUpscaleModel ? imageStore.sourceImages.length === 0 : !imageStore.prompt.trim() || (imageStore.isEditModel && imageStore.sourceImages.length === 0))}
					class="flex w-full items-center justify-center gap-2 rounded-full bg-text-primary px-3 py-2 text-[13px] font-medium text-bg transition-opacity duration-100 not-disabled:hover:bg-text-secondary disabled:opacity-40"
				>
					{#if imageStore.generating}
						<Icon src={FiLoader} size="14" className="animate-spin" />
						Processing...
					{:else if imageStore.isUpscaleModel}
						<Icon src={FiArrowUp} size="14" />
						Upscale
					{:else if imageStore.isEditModel}
						<Icon src={FiEdit2} size="14" />
						Edit
					{:else}
						<Icon src={FiZap} size="14" />
						Generate
					{/if}
				</button>
			</div>
		</form>
	</div>

	<!-- Gallery -->
	<div class="flex-1 overflow-y-auto">
		{#if reversedJobs.length === 0}
			<div class="flex h-full items-center justify-center">
				<span class="text-[13px] text-text-muted">No images generated yet.</span>
			</div>
		{:else}
			<div class="flex flex-col gap-6 p-4">
				{#each reversedJobs as job (job.id)}
					<div class="flex flex-col gap-2">
						<div class="flex items-center gap-2">
							{#if job.status === "error"}
								<Icon src={FiAlertCircle} size="13" className="text-error" />
							{/if}
							<span class="text-[12px] font-medium text-text-secondary">{job.model}</span>
							{#if job.type === "generate"}
								<span class="text-[11px] text-text-muted">&middot;</span>
								<span class="text-[12px] text-text-muted">{job.width}&times;{job.height}</span>
							{/if}
							{#if job.type === "edit"}
								<span class="text-[11px] text-text-muted">&middot;</span>
								<span class="text-[11px] text-text-muted">edit</span>
							{/if}
							{#if job.type === "upscale"}
								<span class="text-[11px] text-text-muted">&middot;</span>
								<span class="text-[11px] text-text-muted">upscale</span>
								{#if job.width && job.height}
									<span class="text-[11px] text-text-muted">&middot;</span>
									<span class="text-[12px] text-text-muted">{job.width}&times;{job.height}</span>
								{/if}
							{/if}
						</div>
						<div class="flex items-start gap-2">
							{#if job.prompt}
								<p class="flex-1 text-[12px] leading-relaxed text-text-secondary line-clamp-2">{job.prompt}</p>
							{/if}
							<div class="ml-auto shrink-0">
							{#if job.status === "pending"}
								<button
									onclick={() => imageStore.removeJob(job.id)}
									class="flex shrink-0 items-center gap-1 rounded px-1.5 py-0.5 text-[11px] text-text-muted transition-colors duration-100 hover:bg-overlay-light hover:text-danger"
									title="Cancel"
								>
									<Icon src={FiX} size="12" />
									Cancel
								</button>
							{:else}
								<button
									onclick={() => imageStore.removeJob(job.id)}
									class="flex shrink-0 items-center gap-1 rounded px-1.5 py-0.5 text-text-muted transition-colors duration-100 hover:bg-overlay-light hover:text-danger"
									title="Delete all"
								>
									<Icon src={FiTrash2} size="12" />
								</button>
							{/if}
							</div>
						</div>
						{#if job.status === "error" && job.error}
							<p class="text-[12px] text-error">{job.error}</p>
						{/if}
						{#if job.source_images && job.source_images > 0}
							<div class="flex items-center gap-1.5">
								<span class="text-[11px] text-text-muted">Source:</span>
								{#each Array(job.source_images) as _, i}
									<img
										src={imageSourceUrl(job.id, i)}
										alt="Source {i + 1}"
										class="h-10 w-10 rounded border border-border object-cover"
										loading="lazy"
									/>
								{/each}
							</div>
						{/if}
						{#if job.status === "pending" || job.status === "generating"}
							<div class="flex flex-wrap gap-2">
								{#each Array(job.count) as _}
									<div class="h-48 w-48 animate-pulse rounded-lg bg-surface"></div>
								{/each}
							</div>
						{/if}
						{#if job.image_count > 0}
							<div class="flex flex-wrap gap-2">
								{#each Array(job.image_count) as _, i}
									<div class="group relative">
										<button
											onclick={() => openLightbox(job.id, i)}
											class="overflow-hidden rounded-lg border border-border bg-bg-tertiary transition-[border-color] duration-100 hover:border-border-light"
										>
											<img
												src={imageResultUrl(job.id, i)}
												alt="{job.prompt} ({i + 1})"
												class="h-48 w-48 object-cover"
												loading="lazy"
											/>
										</button>
										<div class="absolute right-1.5 top-1.5 flex gap-1 opacity-0 transition-opacity duration-100 group-hover:opacity-100">
											<button
												onclick={() => imageStore.useAsSource(job.id, i)}
												class="flex h-7 w-7 items-center justify-center rounded-md bg-black/60 text-white backdrop-blur-sm transition-colors duration-100 hover:bg-black/80"
												title="Use as source"
											>
												<Icon src={FiEdit2} size="13" />
											</button>
											{#if imageStore.upscaleModels.length > 0}
												<div class="relative">
													<button
														onclick={(e) => { e.stopPropagation(); upscaleMenu = upscaleMenu?.jobId === job.id && upscaleMenu?.index === i ? null : { jobId: job.id, index: i }; }}
														class="flex h-7 w-7 items-center justify-center rounded-md bg-black/60 text-white backdrop-blur-sm transition-colors duration-100 hover:bg-black/80"
														title="Upscale"
													>
														<Icon src={FiArrowUp} size="13" />
													</button>
													{#if upscaleMenu?.jobId === job.id && upscaleMenu?.index === i}
														<div class="absolute right-0 top-8 z-10 min-w-35 rounded-md border border-border bg-bg-secondary py-1 shadow-elevated">
															{#each imageStore.upscaleModels as uModel}
																<button
																	onclick={(e) => { e.stopPropagation(); if (uModel === "seedvr2") { imageStore.useForUpscale(job.id, i, uModel); } else { imageStore.upscale(job.id, i, uModel); } upscaleMenu = null; }}
																	class="block w-full px-3 py-1.5 text-left text-[12px] text-text-secondary hover:bg-overlay-light"
																>
																	{uModel}
																</button>
															{/each}
														</div>
													{/if}
												</div>
											{/if}
											<a
												href={imageResultUrl(job.id, i)}
												download="{job.id}_{i}.png"
												class="flex h-7 w-7 items-center justify-center rounded-md bg-black/60 text-white backdrop-blur-sm transition-colors duration-100 hover:bg-black/80"
												title="Download"
											>
												<Icon src={FiDownload} size="13" />
											</a>
											<button
												onclick={() => imageStore.removeImage(job.id, i)}
												class="flex h-7 w-7 items-center justify-center rounded-md bg-black/60 text-white backdrop-blur-sm transition-colors duration-100 hover:bg-red-600"
												title="Delete"
											>
												<Icon src={FiTrash2} size="13" />
											</button>
										</div>
									</div>
								{/each}
							</div>
						{/if}
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>

<!-- Lightbox -->
{#if lightboxUrl}
	<button
		class="fixed inset-0 z-50 flex cursor-zoom-out items-center justify-center bg-black/80 backdrop-blur-sm"
		onclick={closeLightbox}
	>
		<img
			src={lightboxUrl}
			alt="Full size"
			class="max-h-[90vh] max-w-[90vw] rounded-lg shadow-elevated"
		/>
	</button>
{/if}

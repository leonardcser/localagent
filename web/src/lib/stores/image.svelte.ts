import {
	getImageModels,
	getImageJobs,
	submitImageJob,
	submitImageEditJob,
	submitImageUpscaleJob,
	deleteImageJob,
	deleteImageResult,
	unloadImageModel,
	imageResultUrl,
	type ImageJob,
	type ImageGenerateParams,
	type ImageModelsResponse,
} from "$lib/api";

function createImageStore() {
	let modelData = $state<ImageModelsResponse>({
		generate: [],
		edit: [],
		upscale: [],
	});
	let jobs = $state<ImageJob[]>([]);
	let selectedModel = $state("");
	let prompt = $state("");
	let negativePrompt = $state("");
	let width = $state(1024);
	let height = $state(1024);
	let seed = $state("");
	let steps = $state("");
	let guidanceScale = $state("");
	let scale = $state("");
	let count = $state(1);
	let generating = $state(false);
	let unloading = $state(false);
	let sourceImages = $state<File[]>([]);
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	let models = $derived([
		...modelData.generate,
		...modelData.edit,
		...modelData.upscale,
	]);
	let upscaleModels = $derived(modelData.upscale);
	let isEditModel = $derived(modelData.edit.includes(selectedModel));
	let isUpscaleModel = $derived(modelData.upscale.includes(selectedModel));
	let loadedModel = $derived(modelData.loaded_model ?? null);

	async function fetchModels() {
		modelData = await getImageModels();
		const all = [
			...modelData.generate,
			...modelData.edit,
			...modelData.upscale,
		];
		if (all.length > 0 && !selectedModel) {
			selectedModel = all[0];
		}
	}

	async function loadJobs() {
		jobs = await getImageJobs();
	}

	async function generate() {
		if (!selectedModel) return;

		const isUpscale = modelData.upscale.includes(selectedModel);
		if (!isUpscale && !prompt.trim()) return;
		if (
			(isUpscale || modelData.edit.includes(selectedModel)) &&
			sourceImages.length === 0
		)
			return;

		generating = true;

		let id: string | null;

		if (isUpscale) {
			const form = new FormData();
			form.append("model", selectedModel);
			for (const file of sourceImages) {
				form.append("images[]", file);
			}
			if (scale.trim()) {
				form.append("scale", scale.trim());
			}
			id = await submitImageUpscaleJob(form);
		} else if (sourceImages.length > 0) {
			const form = new FormData();
			form.append("model", selectedModel);
			form.append("prompt", prompt.trim());
			if (negativePrompt.trim()) {
				form.append("negative_prompt", negativePrompt.trim());
			}
			if (seed.trim()) {
				form.append("seed", seed.trim());
			}
			if (steps.trim()) {
				form.append("steps", steps.trim());
			}
			if (guidanceScale.trim()) {
				form.append("guidance_scale", guidanceScale.trim());
			}
			form.append("count", String(count));
			for (const file of sourceImages) {
				form.append("images[]", file);
			}
			id = await submitImageEditJob(form);
		} else {
			const params: ImageGenerateParams = {
				model: selectedModel,
				prompt: prompt.trim(),
				width,
				height,
				count,
			};
			if (negativePrompt.trim()) {
				params.negative_prompt = negativePrompt.trim();
			}
			if (seed.trim()) {
				const s = parseInt(seed.trim(), 10);
				if (!isNaN(s)) params.seed = s;
			}
			if (steps.trim()) {
				const s = parseInt(steps.trim(), 10);
				if (!isNaN(s)) params.steps = s;
			}
			if (guidanceScale.trim()) {
				const s = parseFloat(guidanceScale.trim());
				if (!isNaN(s)) params.guidance_scale = s;
			}
			id = await submitImageJob(params);
		}

		generating = false;

		if (id) {
			startPolling();
			await loadJobs();
		}
	}

	function startPolling() {
		if (pollTimer) return;
		pollTimer = setInterval(async () => {
			const before = new Set(
				jobs
					.filter((j) => j.status === "pending" || j.status === "generating")
					.map((j) => j.id),
			);
			await loadJobs();
			const hasPending = jobs.some(
				(j) => j.status === "pending" || j.status === "generating",
			);
			if (before.size > 0 && !hasPending) fetchModels();
			if (!hasPending) stopPolling();
		}, 2000);
	}

	function stopPolling() {
		if (pollTimer) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	async function init() {
		await Promise.all([fetchModels(), loadJobs()]);
		const hasPending = jobs.some(
			(j) => j.status === "pending" || j.status === "generating",
		);
		if (hasPending) startPolling();
	}

	async function removeJob(id: string) {
		await deleteImageJob(id);
		jobs = jobs.filter((j) => j.id !== id);
	}

	async function removeImage(jobId: string, index: number) {
		const remaining = await deleteImageResult(jobId, index);
		if (remaining === null) return;
		if (remaining === 0) {
			jobs = jobs.filter((j) => j.id !== jobId);
		} else {
			await loadJobs();
		}
	}

	async function useAsSource(jobId: string, index: number) {
		const url = imageResultUrl(jobId, index);
		try {
			const res = await fetch(url);
			if (!res.ok) return;
			const blob = await res.blob();
			const file = new File([blob], `source_${jobId}_${index}.png`, {
				type: "image/png",
			});

			if (modelData.edit.length > 0) {
				selectedModel = modelData.edit[0];
			}
			sourceImages = [file];
		} catch {
			// ignore fetch errors
		}
	}

	async function useForUpscale(jobId: string, index: number, model: string) {
		const url = imageResultUrl(jobId, index);
		try {
			const res = await fetch(url);
			if (!res.ok) return;
			const blob = await res.blob();
			const file = new File([blob], `source_${jobId}_${index}.png`, {
				type: "image/png",
			});
			selectedModel = model;
			sourceImages = [file];
		} catch {
			// ignore fetch errors
		}
	}

	async function upscale(jobId: string, index: number, model: string) {
		const url = imageResultUrl(jobId, index);
		try {
			const res = await fetch(url);
			if (!res.ok) return;
			const blob = await res.blob();
			const file = new File([blob], `source_${jobId}_${index}.png`, {
				type: "image/png",
			});

			const form = new FormData();
			form.append("model", model);
			form.append("images[]", file);

			const id = await submitImageUpscaleJob(form);
			if (id) {
				startPolling();
				await loadJobs();
			}
		} catch {
			// ignore fetch errors
		}
	}

	function addSourceImages(files: File[]) {
		sourceImages = [...sourceImages, ...files];
	}

	function removeSourceImage(index: number) {
		sourceImages = sourceImages.filter((_, i) => i !== index);
	}

	function clearSourceImages() {
		sourceImages = [];
	}

	async function unload() {
		unloading = true;
		await unloadImageModel();
		await fetchModels();
		unloading = false;
	}

	function destroy() {
		stopPolling();
	}

	return {
		get models() {
			return models;
		},
		get jobs() {
			return jobs;
		},
		get selectedModel() {
			return selectedModel;
		},
		set selectedModel(v: string) {
			if (v !== selectedModel) {
				if (!modelData.edit.includes(v) && !modelData.upscale.includes(v)) {
					sourceImages = [];
				}
			}
			selectedModel = v;
		},
		get prompt() {
			return prompt;
		},
		set prompt(v: string) {
			prompt = v;
		},
		get negativePrompt() {
			return negativePrompt;
		},
		set negativePrompt(v: string) {
			negativePrompt = v;
		},
		get width() {
			return width;
		},
		set width(v: number) {
			width = v;
		},
		get height() {
			return height;
		},
		set height(v: number) {
			height = v;
		},
		get seed() {
			return seed;
		},
		set seed(v: string) {
			seed = v;
		},
		get steps() {
			return steps;
		},
		set steps(v: string) {
			steps = v;
		},
		get guidanceScale() {
			return guidanceScale;
		},
		set guidanceScale(v: string) {
			guidanceScale = v;
		},
		get count() {
			return count;
		},
		set count(v: number) {
			count = v;
		},
		get generating() {
			return generating;
		},
		get sourceImages() {
			return sourceImages;
		},
		get isEditModel() {
			return isEditModel;
		},
		get isUpscaleModel() {
			return isUpscaleModel;
		},
		get scale() {
			return scale;
		},
		set scale(v: string) {
			scale = v;
		},
		get modelGroups() {
			return modelData;
		},
		get upscaleModels() {
			return upscaleModels;
		},
		get loadedModel() {
			return loadedModel;
		},
		get unloading() {
			return unloading;
		},
		init,
		unload,
		generate,
		removeJob,
		removeImage,
		useAsSource,
		useForUpscale,
		upscale,
		addSourceImages,
		removeSourceImage,
		clearSourceImages,
		destroy,
	};
}

export const imageStore = createImageStore();

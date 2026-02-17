import {
	getImageModels,
	getImageJobs,
	submitImageJob,
	submitImageEditJob,
	deleteImageJob,
	deleteImageResult,
	imageResultUrl,
	type ImageJob,
	type ImageGenerateParams,
} from "$lib/api";

function createImageStore() {
	let models = $state<string[]>([]);
	let jobs = $state<ImageJob[]>([]);
	let selectedModel = $state("");
	let prompt = $state("");
	let negativePrompt = $state("");
	let width = $state(1024);
	let height = $state(1024);
	let seed = $state("");
	let count = $state(1);
	let generating = $state(false);
	let sourceImages = $state<File[]>([]);
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	let isEditModel = $derived(selectedModel.includes("edit"));

	async function fetchModels() {
		models = await getImageModels();
		if (models.length > 0 && !selectedModel) {
			selectedModel = models[0];
		}
	}

	async function loadJobs() {
		jobs = await getImageJobs();
	}

	async function generate() {
		if (!prompt.trim() || !selectedModel) return;
		generating = true;

		let id: string | null;

		if (sourceImages.length > 0) {
			const form = new FormData();
			form.append("model", selectedModel);
			form.append("prompt", prompt.trim());
			if (negativePrompt.trim()) {
				form.append("negative_prompt", negativePrompt.trim());
			}
			if (seed.trim()) {
				form.append("seed", seed.trim());
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
			await loadJobs();
			const hasPending = jobs.some(
				(j) => j.status === "pending" || j.status === "generating",
			);
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

			const editModel = models.find((m) => m.includes("edit"));
			if (editModel) {
				selectedModel = editModel;
			}
			sourceImages = [file];
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
				if (!v.includes("edit")) {
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
		init,
		generate,
		removeJob,
		removeImage,
		useAsSource,
		addSourceImages,
		removeSourceImage,
		clearSourceImages,
		destroy,
	};
}

export const imageStore = createImageStore();

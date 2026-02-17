<script lang="ts">
import { onDestroy } from "svelte";

interface Props {
	mode: "recording" | "transcribing";
	stream: MediaStream | null;
}

let { mode, stream }: Props = $props();

let canvas: HTMLCanvasElement;
let samples: number[] = [];
let animId: number;
let audioCtx: AudioContext | null = null;
let analyser: AnalyserNode | null = null;
let source: MediaStreamAudioSourceNode | null = null;

const BAR_WIDTH = 2;
const BAR_GAP = 2;
const GAIN = 4;
const SMOOTHING = 0.7;
const SAMPLE_INTERVAL = 50;
let lastSampleTime = 0;
let smoothedVolume = 0;

function setupAudio(s: MediaStream) {
	cleanupAudio();
	audioCtx = new AudioContext();
	analyser = audioCtx.createAnalyser();
	analyser.fftSize = 256;
	source = audioCtx.createMediaStreamSource(s);
	source.connect(analyser);
}

function cleanupAudio() {
	source?.disconnect();
	audioCtx?.close();
	audioCtx = null;
	analyser = null;
	source = null;
}

function getVolume(): number {
	if (!analyser) return 0;
	const data = new Uint8Array(analyser.frequencyBinCount);
	analyser.getByteTimeDomainData(data);
	let sum = 0;
	for (let i = 0; i < data.length; i++) {
		const v = (data[i] - 128) / 128;
		sum += v * v;
	}
	const raw = Math.min(1, Math.sqrt(sum / data.length) * GAIN);
	smoothedVolume += (raw - smoothedVolume) * SMOOTHING;
	return smoothedVolume;
}

function draw(time: number) {
	if (!canvas) return;
	const ctx = canvas.getContext("2d");
	if (!ctx) return;

	const dpr = window.devicePixelRatio || 1;
	const rect = canvas.getBoundingClientRect();
	canvas.width = rect.width * dpr;
	canvas.height = rect.height * dpr;
	ctx.scale(dpr, dpr);

	const w = rect.width;
	const h = rect.height;
	const maxBars = Math.floor(w / (BAR_WIDTH + BAR_GAP));

	if (mode === "recording" && time - lastSampleTime >= SAMPLE_INTERVAL) {
		lastSampleTime = time;
		samples.push(getVolume());
		if (samples.length > maxBars) {
			samples = samples.slice(samples.length - maxBars);
		}
	}

	ctx.clearRect(0, 0, w, h);

	const barCount = Math.min(samples.length, maxBars);

	if (mode === "transcribing") {
		const phase = time * 0.002;
		for (let i = 0; i < barCount; i++) {
			const x = w - (barCount - i) * (BAR_WIDTH + BAR_GAP);
			const base = samples[samples.length - barCount + i] || 0;
			const pulse = 0.3 + 0.7 * (0.5 + 0.5 * Math.sin(phase + i * 0.3));
			const barH = Math.max(2, base * pulse * (h * 0.85));
			const y = (h - barH) / 2;
			ctx.fillStyle = "rgba(255, 255, 255, 0.4)";
			ctx.beginPath();
			ctx.roundRect(x, y, BAR_WIDTH, barH, 1);
			ctx.fill();
		}
	} else {
		for (let i = 0; i < barCount; i++) {
			const x = w - (barCount - i) * (BAR_WIDTH + BAR_GAP);
			const vol = samples[samples.length - barCount + i] || 0;
			const barH = Math.max(2, vol * (h * 0.85));
			const y = (h - barH) / 2;
			ctx.fillStyle = "rgba(255, 255, 255, 0.85)";
			ctx.beginPath();
			ctx.roundRect(x, y, BAR_WIDTH, barH, 1);
			ctx.fill();
		}
	}

	animId = requestAnimationFrame(draw);
}

$effect(() => {
	if (stream) {
		setupAudio(stream);
	}
});

$effect(() => {
	if (canvas) {
		samples = [];
		lastSampleTime = 0;
		smoothedVolume = 0;
		animId = requestAnimationFrame(draw);
		return () => cancelAnimationFrame(animId);
	}
});

onDestroy(() => {
	cancelAnimationFrame(animId);
	cleanupAudio();
});
</script>

<canvas bind:this={canvas} class="h-full w-full"></canvas>

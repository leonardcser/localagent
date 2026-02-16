import {
	connectSSE,
	getHistory,
	getMockTimeline,
	type ActivityEventData,
	type HistoryMessage,
	sendMessage,
	uploadFile,
} from "$lib/api";

export type TimelineItem =
	| { kind: "message"; id: number; role: string; content: string; timestamp: string }
	| ({ kind: "activity"; id: number } & ActivityEventData);

let nextId = 0;

function createChat() {
	let timeline = $state<TimelineItem[]>([]);
	let input = $state("");
	let loading = $state(false);
	let recording = $state(false);
	let pendingMedia = $state<string[]>([]);
	let dragging = $state(false);
	let eventSource: EventSource | null = null;
	let mediaRecorder: MediaRecorder | null = null;

	function now() {
		return new Date().toLocaleTimeString("en-GB", { hour12: false });
	}

	function addMessage(msg: HistoryMessage) {
		if (!msg.content) return;
		timeline.push({ kind: "message", ...msg, timestamp: now(), id: ++nextId });
	}

	function addActivity(evt: ActivityEventData) {
		timeline.push({ kind: "activity", ...evt, id: ++nextId });
	}

	async function init() {
		const mockItems = getMockTimeline();
		if (mockItems.length > 0) {
			timeline = mockItems.map((item) => ({ ...item, id: ++nextId }));
		} else {
			try {
				const history = await getHistory();
				timeline = history
					.filter((m) => m.content)
					.map((m) => ({
						kind: "message" as const,
						...m,
						timestamp: now(),
						id: ++nextId,
					}));
			} catch {
				// ignore
			}
		}

		eventSource = connectSSE(
			(msg) => {
				loading = false;
				addMessage(msg);
			},
			(evt) => {
				addActivity(evt);
			},
		);
	}

	async function send() {
		const content = input.trim();
		const media = [...pendingMedia];
		if (!content && media.length === 0) return;

		addMessage({ role: "user", content });
		input = "";
		pendingMedia = [];
		loading = true;

		try {
			await sendMessage(content, media);
		} catch {
			loading = false;
		}
	}

	async function toggleRecording() {
		if (recording) {
			mediaRecorder?.stop();
			mediaRecorder = null;
			recording = false;
			return;
		}
		try {
			const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
			const chunks: Blob[] = [];
			mediaRecorder = new MediaRecorder(stream);
			mediaRecorder.ondataavailable = (e) => {
				if (e.data.size > 0) chunks.push(e.data);
			};
			mediaRecorder.onstop = async () => {
				stream.getTracks().forEach((t) => t.stop());
				const blob = new Blob(chunks, { type: "audio/webm" });
				const file = new File([blob], "voice.webm", { type: "audio/webm" });
				const path = await uploadFile(file, "voice");
				if (path) pendingMedia.push(path);
			};
			mediaRecorder.start();
			recording = true;
		} catch {
			// mic access denied
		}
	}

	async function attachFiles(files: FileList) {
		for (const file of files) {
			const path = await uploadFile(file);
			if (path) pendingMedia.push(path);
		}
	}

	async function handleDrop(files: FileList) {
		dragging = false;
		await attachFiles(files);
	}

	function removeMedia(index: number) {
		pendingMedia.splice(index, 1);
	}

	function destroy() {
		eventSource?.close();
	}

	return {
		get timeline() {
			return timeline;
		},
		get input() {
			return input;
		},
		set input(v: string) {
			input = v;
		},
		get loading() {
			return loading;
		},
		get recording() {
			return recording;
		},
		get pendingMedia() {
			return pendingMedia;
		},
		get dragging() {
			return dragging;
		},
		set dragging(v: boolean) {
			dragging = v;
		},
		init,
		send,
		toggleRecording,
		attachFiles,
		handleDrop,
		removeMedia,
		destroy,
	};
}

export const chat = createChat();

import {
	connectSSE,
	getHistory,
	getMockTimeline,
	type ActivityEventData,
	type HistoryMessage,
	sendMessage,
	uploadFile,
} from "$lib/api";
import { nowTimestamp } from "$lib/utils";

export type TimelineItem =
	| { kind: "message"; id: number; role: string; content: string; timestamp: string; media?: string[] }
	| ({ kind: "activity"; id: number } & ActivityEventData);

export type MessageTimelineItem = Extract<TimelineItem, { kind: "message" }>;
export type ActivityTimelineItem = Extract<TimelineItem, { kind: "activity" }>;

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

	function addMessage(msg: HistoryMessage) {
		if (!msg.content) return;
		timeline.push({ kind: "message", ...msg, timestamp: nowTimestamp(), id: ++nextId });
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
				timeline = history.items
					.map((item): TimelineItem | null => {
						if (item.type === "message") {
							if (!item.content) return null;
							return {
								kind: "message",
								role: item.role!,
								content: item.content,
								timestamp: item.timestamp,
								media: item.media,
								id: ++nextId,
							};
						}
						return {
							kind: "activity",
							event_type: item.event_type!,
							timestamp: item.timestamp,
							message: item.message!,
							detail: item.detail,
							id: ++nextId,
						};
					})
					.filter((item): item is TimelineItem => item !== null);
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

		timeline.push({ kind: "message", role: "user", content, timestamp: nowTimestamp(), id: ++nextId, media: media.length > 0 ? media : undefined });
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

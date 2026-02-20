import {
	connectSSE,
	getHistory,
	getMockTimeline,
	reportActive,
	transcribeAudio,
	type ActivityEventData,
	type HistoryMessage,
	sendMessage,
	uploadFile,
} from "$lib/api";
import { nowTimestamp } from "$lib/utils";

export type TimelineItem =
	| {
			kind: "message";
			id: number;
			role: string;
			content: string;
			timestamp: string;
			media?: string[];
			queued?: boolean;
	  }
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
	let expandedGroups = $state<Record<string, boolean>>({});
	let clientId: string | null = null;
	let eventSource: EventSource | null = null;
	let mediaRecorder: MediaRecorder | null = null;
	let mediaStream = $state<MediaStream | null>(null);

	function addMessage(msg: HistoryMessage) {
		if (!msg.content && (!msg.media || msg.media.length === 0)) return;
		timeline.push({
			kind: "message",
			...msg,
			timestamp: nowTimestamp(),
			id: ++nextId,
		});
	}

	function addActivity(evt: ActivityEventData) {
		// When the agent starts processing, clear "queued" from the first queued message
		if (evt.event_type === "processing_start") {
			for (const item of timeline) {
				if (item.kind === "message" && item.queued) {
					item.queued = false;
					break;
				}
			}
		}
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
							if (item.role !== "user" && !item.content) return null;
							return {
								kind: "message",
								role: item.role!,
								content: item.content ?? "",
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
			(processing) => {
				loading = processing;
			},
			(id) => {
				clientId = id;
				reportActive(id, document.visibilityState === "visible");
			},
			() => {
				sync();
			},
		);
	}

	async function send() {
		const content = input.trim();
		const media = [...pendingMedia];
		if (!content && media.length === 0) return;

		timeline.push({
			kind: "message",
			role: "user",
			content,
			timestamp: nowTimestamp(),
			id: ++nextId,
			media: media.length > 0 ? media : undefined,
			queued: loading ? true : undefined,
		});
		input = "";
		pendingMedia = [];
		loading = true;

		try {
			await sendMessage(content, media);
		} catch {
			loading = false;
		}
	}

	let transcribing = $state(false);
	let sendAfterTranscribe = false;

	function stopRecording() {
		mediaRecorder?.stop();
		mediaRecorder = null;
		recording = false;
	}

	async function toggleRecording() {
		if (recording) {
			sendAfterTranscribe = false;
			stopRecording();
			return;
		}
		try {
			const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
			mediaStream = stream;
			const chunks: Blob[] = [];
			mediaRecorder = new MediaRecorder(stream);
			mediaRecorder.ondataavailable = (e) => {
				if (e.data.size > 0) chunks.push(e.data);
			};
			mediaRecorder.onstop = async () => {
				stream.getTracks().forEach((t) => t.stop());
				mediaStream = null;
				const blob = new Blob(chunks, { type: "audio/webm" });
				const file = new File([blob], "voice.webm", { type: "audio/webm" });
				transcribing = true;
				const shouldSend = sendAfterTranscribe;
				sendAfterTranscribe = false;
				const text = await transcribeAudio(file);
				transcribing = false;
				if (text) {
					const trimmed = text.trim();
					input = input ? input + " " + trimmed : trimmed;
				}
				if (shouldSend) {
					await send();
				}
			};
			mediaRecorder.start();
			recording = true;
		} catch {
			// mic access denied
		}
	}

	function recordAndSend() {
		if (!recording) return;
		sendAfterTranscribe = true;
		stopRecording();
	}

	async function attachFiles(files: FileList) {
		const snapshot = [...files];
		for (const file of snapshot) {
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

	async function sync() {
		try {
			const history = await getHistory();
			timeline = history.items
				.map((item): TimelineItem | null => {
					if (item.type === "message") {
						if (item.role !== "user" && !item.content) return null;
						return {
							kind: "message",
							role: item.role!,
							content: item.content ?? "",
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

	function reportVisibility() {
		if (clientId) {
			reportActive(clientId, document.visibilityState === "visible");
		}
	}

	function isGroupExpanded(key: string): boolean {
		return !!expandedGroups[key];
	}

	function toggleGroupExpanded(key: string) {
		expandedGroups[key] = !expandedGroups[key];
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
		get transcribing() {
			return transcribing;
		},
		get mediaStream() {
			return mediaStream;
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
		reportVisibility,
		isGroupExpanded,
		toggleGroupExpanded,
		send,
		toggleRecording,
		recordAndSend,
		attachFiles,
		handleDrop,
		removeMedia,
		destroy,
	};
}

export const chat = createChat();

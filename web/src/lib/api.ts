export interface HistoryMessage {
	role: string;
	content: string;
}

export interface HistoryItem {
	type: "message" | "activity";
	role?: string;
	content?: string;
	event_type?: string;
	message?: string;
	detail?: Record<string, unknown>;
	timestamp: string;
}

export interface HistoryResponse {
	summary?: string;
	items: HistoryItem[];
}

export interface ActivityEventData {
	event_type: string;
	timestamp: string;
	message: string;
	detail?: Record<string, unknown>;
}

export type MockTimelineItem =
	| { kind: "message"; role: string; content: string; timestamp: string; media?: string[] }
	| ({ kind: "activity" } & ActivityEventData);

const DEV = import.meta.env.DEV;

// --- Mock data for dev mode ---

const mockTimeline: MockTimelineItem[] = [
	// -- First exchange --
	{
		kind: "message",
		role: "user",
		content: "What files are in the workspace?",
		timestamp: "14:20:11",
		media: ["/uploads/screenshot.png"],
	},
	{
		kind: "activity",
		event_type: "processing_start",
		timestamp: "14:20:12",
		message: "Processing message from web:web-user",
		detail: {
			channel: "web",
			sender: "web-user",
			preview: "What files are in the workspace?",
		},
	},
	{
		kind: "activity",
		event_type: "llm_request",
		timestamp: "14:20:12",
		message: "LLM request #1 (gpt-4o)",
		detail: { iteration: 1, model: "gpt-4o", messages: 2, tools: 12 },
	},
	{
		kind: "activity",
		event_type: "tool_call",
		timestamp: "14:20:14",
		message: "Tool: list_dir",
		detail: { tool: "list_dir", args: '{"path":"."}' },
	},
	{
		kind: "activity",
		event_type: "tool_result",
		timestamp: "14:20:14",
		message: "Tool result: list_dir (success)",
		detail: { tool: "list_dir", status: "success", content_length: 312 },
	},
	{
		kind: "activity",
		event_type: "llm_response",
		timestamp: "14:20:16",
		message: "LLM response #2 (445 chars)",
		detail: { iteration: 2, chars: 445 },
	},
	{
		kind: "activity",
		event_type: "complete",
		timestamp: "14:20:16",
		message: "Complete (2 iterations, 445 chars)",
		detail: { session: "web:default", iterations: 2, length: 445 },
	},
	{
		kind: "message",
		role: "assistant",
		content:
			"I found the following files in your workspace:\n\n```\nsrc/\n  main.go\n  config.go\n  utils/\n    helpers.go\npkg/\n  agent/\n    loop.go\n    context.go\n  tools/\n    shell.go\n    edit.go\ngo.mod\ngo.sum\n```\n\nThe project is a Go application with an agent loop and various tool implementations.",
		timestamp: "14:20:16",
	},

	// -- Second exchange --
	{
		kind: "message",
		role: "user",
		content: "Show me the main.go file",
		timestamp: "14:21:02",
	},
	{
		kind: "activity",
		event_type: "processing_start",
		timestamp: "14:21:03",
		message: "Processing message from web:web-user",
		detail: {
			channel: "web",
			sender: "web-user",
			preview: "Show me the main.go file",
		},
	},
	{
		kind: "activity",
		event_type: "llm_request",
		timestamp: "14:21:03",
		message: "LLM request #1 (gpt-4o)",
		detail: { iteration: 1, model: "gpt-4o", messages: 6, tools: 12 },
	},
	{
		kind: "activity",
		event_type: "tool_call",
		timestamp: "14:21:05",
		message: "Tool: read_file",
		detail: { tool: "read_file", args: '{"path":"src/main.go"}' },
	},
	{
		kind: "activity",
		event_type: "tool_result",
		timestamp: "14:21:05",
		message: "Tool result: read_file (success)",
		detail: { tool: "read_file", status: "success", content_length: 286 },
	},
	{
		kind: "activity",
		event_type: "llm_response",
		timestamp: "14:21:07",
		message: "LLM response #2 (320 chars)",
		detail: { iteration: 2, chars: 320 },
	},
	{
		kind: "activity",
		event_type: "complete",
		timestamp: "14:21:07",
		message: "Complete (2 iterations, 320 chars)",
		detail: { session: "web:default", iterations: 2, length: 320 },
	},
	{
		kind: "message",
		role: "assistant",
		content:
			'Here\'s the contents of `main.go`:\n\n```go\npackage main\n\nimport (\n\t"fmt"\n\t"os"\n)\n\nfunc main() {\n\tif len(os.Args) < 2 {\n\t\tfmt.Println("Usage: localagent <command>")\n\t\tos.Exit(1)\n\t}\n\n\tswitch os.Args[1] {\n\tcase "agent":\n\t\tagentCmd()\n\tcase "gateway":\n\t\tgatewayCmd()\n\tdefault:\n\t\tfmt.Printf("Unknown command: %s\\n", os.Args[1])\n\t}\n}\n```',
		timestamp: "14:21:07",
	},

	// -- Third exchange --
	{
		kind: "message",
		role: "user",
		content: "Can you add a health check endpoint to the gateway?",
		timestamp: "14:23:00",
	},
	{
		kind: "activity",
		event_type: "processing_start",
		timestamp: "14:23:01",
		message: "Processing message from web:web-user",
		detail: {
			channel: "web",
			sender: "web-user",
			preview: "Can you add a health check endpoint to the gateway?",
		},
	},
	{
		kind: "activity",
		event_type: "llm_request",
		timestamp: "14:23:01",
		message: "LLM request #1 (gpt-4o)",
		detail: { iteration: 1, model: "gpt-4o", messages: 8, tools: 12 },
	},
	{
		kind: "activity",
		event_type: "tool_call",
		timestamp: "14:23:03",
		message: "Calling 2 tool(s): read_file, list_dir",
		detail: { tools: ["read_file", "list_dir"], count: 2, iteration: 1 },
	},
	{
		kind: "activity",
		event_type: "tool_call",
		timestamp: "14:23:03",
		message: "Tool: read_file",
		detail: { tool: "read_file", args: '{"path":"pkg/health/server.go"}' },
	},
	{
		kind: "activity",
		event_type: "tool_result",
		timestamp: "14:23:03",
		message: "Tool result: read_file (success)",
		detail: { tool: "read_file", status: "success", content_length: 1842 },
	},
	{
		kind: "activity",
		event_type: "tool_call",
		timestamp: "14:23:03",
		message: "Tool: list_dir",
		detail: { tool: "list_dir", args: '{"path":"pkg/health"}' },
	},
	{
		kind: "activity",
		event_type: "tool_result",
		timestamp: "14:23:04",
		message: "Tool result: list_dir (success)",
		detail: { tool: "list_dir", status: "success", content_length: 256 },
	},
	{
		kind: "activity",
		event_type: "llm_request",
		timestamp: "14:23:04",
		message: "LLM request #2 (gpt-4o)",
		detail: { iteration: 2, model: "gpt-4o", messages: 12, tools: 12 },
	},
	{
		kind: "activity",
		event_type: "tool_call",
		timestamp: "14:23:07",
		message: "Tool: edit_file",
		detail: {
			tool: "edit_file",
			args: '{"path":"pkg/health/server.go","old_string":"func (s *Server) setupRoutes()..."}',
		},
	},
	{
		kind: "activity",
		event_type: "tool_result",
		timestamp: "14:23:07",
		message: "Tool result: edit_file (success)",
		detail: { tool: "edit_file", status: "success", content_length: 45 },
	},
	{
		kind: "activity",
		event_type: "llm_request",
		timestamp: "14:23:08",
		message: "LLM request #3 (gpt-4o)",
		detail: { iteration: 3, model: "gpt-4o", messages: 14, tools: 12 },
	},
	{
		kind: "activity",
		event_type: "llm_response",
		timestamp: "14:23:11",
		message: "LLM response #3 (482 chars)",
		detail: { iteration: 3, chars: 482 },
	},
	{
		kind: "activity",
		event_type: "complete",
		timestamp: "14:23:11",
		message: "Complete (3 iterations, 482 chars)",
		detail: { session: "web:default", iterations: 3, length: 482 },
	},
	{
		kind: "message",
		role: "assistant",
		content:
			'I\'ve added a health check endpoint to the gateway. Here\'s what I did:\n\n1. Created `/api/health` endpoint that returns `200 OK`\n2. Added uptime and version info to the response\n3. The endpoint is available at `http://localhost:8080/api/health`\n\nThe response looks like:\n```json\n{\n  "status": "ok",\n  "version": "0.1.0",\n  "uptime": "2h15m"\n}\n```',
		timestamp: "14:23:11",
	},
];

export function getMockTimeline(): MockTimelineItem[] {
	if (!DEV) return [];
	return mockTimeline;
}

function mockSSE(
	onMessage: (msg: HistoryMessage) => void,
	onActivity: (evt: ActivityEventData) => void,
): EventSource {
	setTimeout(() => {
		onActivity({
			event_type: "processing_start",
			timestamp: new Date().toLocaleTimeString("en-GB", { hour12: false }),
			message: "Processing message from web:web-user",
			detail: { channel: "web", sender: "web-user", preview: "Hello!" },
		});
	}, 3000);

	setTimeout(() => {
		onActivity({
			event_type: "llm_request",
			timestamp: new Date().toLocaleTimeString("en-GB", { hour12: false }),
			message: "LLM request #1 (gpt-4o)",
			detail: { iteration: 1, model: "gpt-4o", messages: 4, tools: 12 },
		});
	}, 3500);

	setTimeout(() => {
		onActivity({
			event_type: "llm_response",
			timestamp: new Date().toLocaleTimeString("en-GB", { hour12: false }),
			message: "LLM response #1 (128 chars)",
			detail: { iteration: 1, chars: 128 },
		});
		onActivity({
			event_type: "complete",
			timestamp: new Date().toLocaleTimeString("en-GB", { hour12: false }),
			message: "Complete (1 iteration, 128 chars)",
			detail: { session: "web:default", iterations: 1, length: 128 },
		});
		onMessage({
			role: "assistant",
			content:
				"Hello! I'm your local agent, ready to help. What would you like to work on?",
		});
	}, 5000);

	return {} as EventSource;
}

// --- Real API ---

export async function sendMessage(
	content: string,
	media: string[],
): Promise<void> {
	if (DEV) return;
	await fetch("/api/messages", {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ content, media }),
	});
}

export async function uploadFile(
	file: File,
	type?: string,
): Promise<string | null> {
	if (DEV) return `/mock/${file.name}`;
	const form = new FormData();
	form.append("file", file);
	if (type) form.append("type", type);
	try {
		const res = await fetch("/api/upload", { method: "POST", body: form });
		const data = await res.json();
		return data.path;
	} catch {
		return null;
	}
}

export async function getHistory(): Promise<HistoryResponse> {
	if (DEV) return { items: [] };
	const res = await fetch("/api/history");
	if (!res.ok) return { items: [] };
	return res.json();
}

export function connectSSE(
	onMessage: (msg: HistoryMessage) => void,
	onActivity: (evt: ActivityEventData) => void,
): EventSource {
	if (DEV) return mockSSE(onMessage, onActivity);

	const es = new EventSource("/api/events");
	es.onmessage = (e) => {
		try {
			const data = JSON.parse(e.data);
			if (data.type === "activity" && data.event) {
				onActivity(data.event);
			} else if (data.role && data.content) {
				onMessage({ role: data.role, content: data.content });
			}
		} catch {
			// ignore parse errors
		}
	};
	es.onerror = () => {
		console.warn("SSE connection error, will reconnect...");
	};
	return es;
}

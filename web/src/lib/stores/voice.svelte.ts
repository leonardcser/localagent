import type { MicVAD } from "@ricky0123/vad-web";

export type VoiceState =
  | "idle"
  | "connecting"
  | "listening"
  | "processing"
  | "speaking";

interface VoiceMessage {
  type: string;
  data?: string;
  text?: string;
  content?: string;
  state?: string;
  message?: string;
  speaker?: string;
  language?: string;
}

const DEV = import.meta.env.DEV;

function createVoiceStore() {
  let state = $state<VoiceState>("idle");
  let transcript = $state("");
  let response = $state("");
  let error = $state("");

  let ws: WebSocket | null = null;
  let vad: MicVAD | null = null;
  let audioCtx: AudioContext | null = null;
  let sampleRate = 24000;
  let scheduledEnd = 0;
  let sourceNodes: AudioBufferSourceNode[] = [];
  let isFirstChunk = false;

  function wsUrl(): string {
    if (DEV) return "ws://localhost:18791/api/voice";
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    return `${proto}//${location.host}/api/voice`;
  }

  async function start() {
    if (state !== "idle") return;
    state = "connecting";
    error = "";
    transcript = "";
    response = "";

    try {
      // Open WebSocket first
      ws = new WebSocket(wsUrl());

      await new Promise<void>((resolve, reject) => {
        ws!.onopen = () => resolve();
        ws!.onerror = () => reject(new Error("WebSocket connection failed"));
        setTimeout(() => reject(new Error("WebSocket timeout")), 10000);
      });

      ws.onmessage = (e) => handleMessage(JSON.parse(e.data));
      ws.onclose = () => {
        if (state !== "idle") stop();
      };
      ws.onerror = () => {};

      // Initialize VAD
      const { MicVAD: MicVADClass } = await import("@ricky0123/vad-web");
      vad = await MicVADClass.new({
        model: "v5",
        baseAssetPath: "/vad/",
        onnxWASMBasePath:
          "https://cdn.jsdelivr.net/npm/onnxruntime-web@1.24.3/dist/",
        onSpeechStart: () => {
          if (state === "speaking" || state === "processing") {
            stopPlayback();
            sendWS({ type: "interrupt" });
          }
        },
        onSpeechEnd: (audio: Float32Array) => {
          if (!ws || ws.readyState !== WebSocket.OPEN) return;
          // Convert Float32Array to base64
          const bytes = new Uint8Array(audio.buffer);
          let binary = "";
          for (let i = 0; i < bytes.length; i++) {
            binary += String.fromCharCode(bytes[i]);
          }
          sendWS({ type: "audio", data: btoa(binary) });
        },
      });

      await vad.start();
      state = "listening";
    } catch (err) {
      error = err instanceof Error ? err.message : "Failed to start voice mode";
      state = "idle";
      cleanup();
    }
  }

  function stop() {
    state = "idle";
    transcript = "";
    response = "";
    cleanup();
  }

  function cleanup() {
    stopPlayback();
    if (vad) {
      vad.destroy();
      vad = null;
    }
    if (ws) {
      if (ws.readyState === WebSocket.OPEN) {
        ws.close();
      }
      ws = null;
    }
    if (audioCtx) {
      audioCtx.close();
      audioCtx = null;
    }
  }

  function handleMessage(msg: VoiceMessage) {
    switch (msg.type) {
      case "status":
        if (
          msg.state === "listening" ||
          msg.state === "processing" ||
          msg.state === "speaking"
        ) {
          state = msg.state;
        }
        if (msg.state === "listening") {
          transcript = "";
          response = "";
        }
        break;

      case "stt":
        transcript = msg.text || "";
        break;

      case "text":
        response = msg.content || "";
        break;

      case "audio_start":
        sampleRate = parseInt(msg.data || "24000", 10);
        if (!audioCtx) {
          audioCtx = new AudioContext();
        }
        if (audioCtx.state === "suspended") {
          audioCtx.resume();
        }
        scheduledEnd = 0;
        isFirstChunk = true;
        break;

      case "audio":
        if (msg.data) playChunk(msg.data);
        break;

      case "audio_end":
        break;

      case "error":
        error = msg.message || "Unknown error";
        break;
    }
  }

  function playChunk(b64: string) {
    if (!audioCtx) {
      audioCtx = new AudioContext();
      scheduledEnd = 0;
    }
    if (audioCtx.state === "suspended") {
      audioCtx.resume();
    }

    // Decode base64 to PCM16 bytes
    const raw = atob(b64);
    const bytes = new Uint8Array(raw.length);
    for (let i = 0; i < raw.length; i++) {
      bytes[i] = raw.charCodeAt(i);
    }

    // Convert PCM16 to Float32
    const numSamples = bytes.length / 2;
    if (numSamples === 0) return;

    const float32 = new Float32Array(numSamples);
    const view = new DataView(bytes.buffer);
    for (let i = 0; i < numSamples; i++) {
      float32[i] = view.getInt16(i * 2, true) / 32768;
    }

    // Fade in the first chunk to avoid a click from silence → non-zero sample
    if (isFirstChunk) {
      const fadeLen = Math.min(Math.floor(sampleRate * 0.01), numSamples);
      for (let i = 0; i < fadeLen; i++) {
        float32[i] *= i / fadeLen;
      }
      isFirstChunk = false;
    }

    const buffer = audioCtx.createBuffer(1, numSamples, sampleRate);
    buffer.getChannelData(0).set(float32);

    const source = audioCtx.createBufferSource();
    source.buffer = buffer;
    source.connect(audioCtx.destination);

    const now = audioCtx.currentTime;
    const startTime = Math.max(now, scheduledEnd);
    source.start(startTime);
    scheduledEnd = startTime + buffer.duration;

    sourceNodes.push(source);
    source.onended = () => {
      const idx = sourceNodes.indexOf(source);
      if (idx >= 0) sourceNodes.splice(idx, 1);
    };
  }

  function stopPlayback() {
    for (const source of sourceNodes) {
      try {
        source.stop();
      } catch {
        // already stopped
      }
    }
    sourceNodes = [];
    scheduledEnd = 0;
  }

  function sendWS(msg: VoiceMessage) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(msg));
    }
  }

  return {
    get state() {
      return state;
    },
    get active() {
      return state !== "idle";
    },
    get transcript() {
      return transcript;
    },
    get response() {
      return response;
    },
    get error() {
      return error;
    },
    start,
    stop,
  };
}

export const voice = createVoiceStore();

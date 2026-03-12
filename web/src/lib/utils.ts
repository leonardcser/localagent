import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function filename(path: string): string {
  return path.split("/").pop() || path;
}

const AUDIO_EXTS = new Set([
  ".mp3",
  ".wav",
  ".ogg",
  ".webm",
  ".m4a",
  ".aac",
  ".flac",
  ".opus",
]);

export function isAudio(path: string): boolean {
  const ext = path.slice(path.lastIndexOf(".")).toLowerCase();
  return AUDIO_EXTS.has(ext);
}

export function mediaUrl(path: string): string {
  return `/api/media/${encodeURIComponent(filename(path))}`;
}

export function nowTimestamp(): string {
  return new Date().toISOString();
}

export function formatTimestamp(ts: string): string {
  const d = new Date(ts);
  if (isNaN(d.getTime())) return ts;
  return formatTime24(d);
}

export function formatTime24(d: Date): string {
  return `${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`;
}

export function getBrowserLocale(): string {
  return navigator.language || "en-US";
}

import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

type WeekStartsOn = 0 | 1 | 2 | 3 | 4 | 5 | 6;

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

  const now = new Date();
  const isToday =
    d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate();

  if (isToday) {
    return formatTime24(d);
  }

  return (
    d.toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
    }) + `, ${formatTime24(d)}`
  );
}

export function formatTime24(d: Date): string {
  return `${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`;
}

export function getBrowserLocale(): string {
  return navigator.language || "en-US";
}

export function getLocaleWeekStartsOn(locale: string): WeekStartsOn {
  try {
    const localeInfo = new Intl.Locale(locale) as Intl.Locale & {
      getWeekInfo?: () => { firstDay: number };
      weekInfo?: { firstDay: number };
    };

    const weekInfo = localeInfo.getWeekInfo?.() ?? localeInfo.weekInfo;
    if (weekInfo?.firstDay !== undefined) {
      return (weekInfo.firstDay % 7) as WeekStartsOn;
    }
  } catch {
    // Fall through to heuristic fallback below.
  }

  try {
    const region = new Intl.Locale(locale).maximize().region;
    if (region === "US") {
      return 0;
    }
  } catch {
    // Ignore and use default fallback.
  }

  return 1;
}

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
	return new Date().toLocaleTimeString("en-GB", { hour12: false });
}

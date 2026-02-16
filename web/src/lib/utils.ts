export function filename(path: string): string {
	return path.split("/").pop() || path;
}

export function nowTimestamp(): string {
	return new Date().toLocaleTimeString("en-GB", { hour12: false });
}

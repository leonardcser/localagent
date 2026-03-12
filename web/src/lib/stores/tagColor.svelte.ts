const STORAGE_KEY = "tag-colors";

const PALETTE = [
  "#ef4444", // red
  "#f97316", // orange
  "#eab308", // yellow
  "#22c55e", // green
  "#06b6d4", // cyan
  "#3b82f6", // blue
  "#8b5cf6", // violet
  "#ec4899", // pink
  "#a855f7", // purple
  "#14b8a6", // teal
];

function load(): Record<string, string> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw);
  } catch {
    // ignore
  }
  return {};
}

function save(colors: Record<string, string>) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(colors));
  } catch {
    // ignore
  }
}

function createTagColorStore() {
  let colors = $state<Record<string, string>>(load());

  return {
    palette: PALETTE,
    get colors() {
      return colors;
    },
    get(tag: string): string | undefined {
      return colors[tag];
    },
    set(tag: string, color: string) {
      colors = { ...colors, [tag]: color };
      save(colors);
    },
    remove(tag: string) {
      const { [tag]: _, ...rest } = colors;
      colors = rest;
      save(colors);
    },
  };
}

export const tagColorStore = createTagColorStore();

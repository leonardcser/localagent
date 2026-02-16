import { createHighlighter, type Highlighter } from "shiki";

let highlighter: Highlighter | null = null;
let initPromise: Promise<Highlighter> | null = null;

const THEME = "vitesse-dark";
const LANGS = [
	"javascript",
	"typescript",
	"python",
	"go",
	"bash",
	"json",
	"html",
	"css",
	"yaml",
	"sql",
	"rust",
	"markdown",
];

function getHighlighter(): Promise<Highlighter> {
	if (highlighter) return Promise.resolve(highlighter);
	if (initPromise) return initPromise;

	initPromise = createHighlighter({ themes: [THEME], langs: LANGS }).then(
		(h) => {
			highlighter = h;
			return h;
		},
	);

	return initPromise;
}

export async function highlightCode(
	code: string,
	lang?: string,
): Promise<string> {
	const h = await getHighlighter();
	const resolvedLang =
		lang && h.getLoadedLanguages().includes(lang) ? lang : "text";
	return h.codeToHtml(code, { lang: resolvedLang, theme: THEME });
}

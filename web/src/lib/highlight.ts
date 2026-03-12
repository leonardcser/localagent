import { createHighlighter, type Highlighter } from "shiki";

let highlighter: Highlighter | null = null;
let initPromise: Promise<Highlighter> | null = null;

const THEME_DARK = "vitesse-dark";
const THEME_LIGHT = "vitesse-light";
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

  initPromise = createHighlighter({
    themes: [THEME_DARK, THEME_LIGHT],
    langs: LANGS,
  }).then((h) => {
    highlighter = h;
    return h;
  });

  return initPromise;
}

export async function highlightCode(
  code: string,
  lang?: string,
): Promise<string> {
  const h = await getHighlighter();
  const resolvedLang =
    lang && h.getLoadedLanguages().includes(lang) ? lang : "text";
  const isLight = document.documentElement.classList.contains("light");
  const theme = isLight ? THEME_LIGHT : THEME_DARK;
  return h.codeToHtml(code, { lang: resolvedLang, theme });
}

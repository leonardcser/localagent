import { Marked } from "marked";
import { highlightCode } from "./highlight";

export const COPY_SVG = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>`;
export const CHECK_SVG = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>`;

const codeBlocks: Map<
	string,
	{ promise: Promise<string>; text: string; lang: string }
> = new Map();
let blockCounter = 0;

function makePlaceholder(id: number): string {
	return `<!--shiki-${id}-->`;
}

function escapeHtml(s: string): string {
	return s
		.replace(/&/g, "&amp;")
		.replace(/</g, "&lt;")
		.replace(/>/g, "&gt;")
		.replace(/"/g, "&quot;");
}

const marked = new Marked({
	renderer: {
		code({ text, lang }) {
			const id = blockCounter++;
			const placeholder = makePlaceholder(id);
			codeBlocks.set(placeholder, {
				promise: highlightCode(text, lang || undefined),
				text,
				lang: lang || "",
			});
			return placeholder;
		},
	},
});

export async function renderMarkdown(source: string): Promise<string> {
	blockCounter = 0;
	codeBlocks.clear();

	let html = await marked.parse(source);

	for (const [placeholder, { promise, text, lang }] of codeBlocks) {
		const highlighted = await promise;
		const escapedText = escapeHtml(text);
		const langLabel = lang
			? `<span class="code-lang">${escapeHtml(lang)}</span>`
			: "";
		const wrapper = `<div class="code-block-wrapper"><div class="code-header">${langLabel}<button class="copy-btn" data-code="${escapedText}" title="Copy code">${COPY_SVG}</button></div>${highlighted}</div>`;
		html = html.replace(placeholder, wrapper);
	}

	codeBlocks.clear();
	return html;
}

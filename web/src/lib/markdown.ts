import { Marked } from "marked";
import { highlightCode } from "./highlight";

export const COPY_SVG = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>`;
export const CHECK_SVG = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>`;

type CodeBlock = { promise: Promise<string>; text: string; lang: string };

let nextBlockId = 0;
let activeBlocks: Map<string, CodeBlock> | null = null;

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

const inlineMarked = new Marked();

function parseInline(text: string): string {
	return inlineMarked.parseInline(text) as string;
}

const marked = new Marked({
	renderer: {
		code({ text, lang }) {
			const id = nextBlockId++;
			const placeholder = makePlaceholder(id);
			activeBlocks!.set(placeholder, {
				promise: highlightCode(text, lang || undefined),
				text,
				lang: lang || "",
			});
			return placeholder;
		},
		table(token) {
			const headerCells = (token.header as Array<{ text: string }>)
				.map((h) => `<th>${parseInline(h.text)}</th>`)
				.join("");
			const bodyRows = (token.rows as Array<Array<{ text: string }>>)
				.map(
					(row) =>
						`<tr>${row.map((cell) => `<td>${parseInline(cell.text)}</td>`).join("")}</tr>`,
				)
				.join("");
			return `<div class="table-wrapper"><table><thead><tr>${headerCells}</tr></thead><tbody>${bodyRows}</tbody></table></div>`;
		},
	},
});

export async function renderMarkdown(source: string): Promise<string> {
	const blocks: Map<string, CodeBlock> = new Map();
	activeBlocks = blocks;

	let html = await marked.parse(source);

	for (const [placeholder, { promise, text, lang }] of blocks) {
		const highlighted = await promise;
		const escapedText = escapeHtml(text);
		const langLabel = lang
			? `<span class="code-lang">${escapeHtml(lang)}</span>`
			: "";
		const wrapper = `<div class="code-block-wrapper"><div class="code-header">${langLabel}<button class="copy-btn" data-code="${escapedText}" title="Copy code">${COPY_SVG}</button></div>${highlighted}</div>`;
		html = html.replace(placeholder, wrapper);
	}

	return html;
}

// Read-only rendering of block documents (docs/realtime.md#document-model)
// as escaped HTML: comments, archived issues, and anything shown without
// the editor. The editor (horizon/editor/view.js) renders its own DOM.

import { html, sheet } from "horizon/core/dom.js";

export const docStyles = sheet(`
  .doc { overflow-wrap: anywhere; }
  .doc > :first-child { margin-block-start: 0; }
  .doc > :last-child { margin-block-end: 0; }
  .doc p { margin: 0 0 var(--lds-space-2); min-block-size: 1.4em; }
  .doc h2, .doc h3, .doc h4 { margin: var(--lds-space-4) 0 var(--lds-space-2); line-height: 1.3; }
  .doc h2 { font-size: var(--lds-font-size-xl); }
  .doc h3 { font-size: var(--lds-font-size-lg); }
  .doc h4 { font-size: var(--lds-font-size-md); }
  .doc ul, .doc ol { margin: 0 0 var(--lds-space-2); padding-inline-start: var(--lds-space-6); }
  .doc li > p { margin: 0; }
  .doc ul.tasks { list-style: none; padding-inline-start: var(--lds-space-2); }
  .doc ul.tasks li { display: flex; gap: var(--lds-space-2); align-items: baseline; }
  .doc ul.tasks li[data-checked="true"] > div { color: var(--lds-color-fg-muted); text-decoration: line-through; }
  .doc blockquote { margin: 0 0 var(--lds-space-2); padding-inline-start: var(--lds-space-4);
    border-inline-start: 3px solid var(--lds-color-border-strong); color: var(--lds-color-fg-muted); }
  .doc .callout { margin: 0 0 var(--lds-space-2); padding: var(--lds-space-3) var(--lds-space-4); border-radius: var(--lds-radius-md);
    background: var(--lds-color-info-subtle); }
  .doc .callout[data-tone="success"] { background: var(--lds-color-success-subtle); }
  .doc .callout[data-tone="warning"] { background: var(--lds-color-warning-subtle); }
  .doc .callout[data-tone="danger"] { background: var(--lds-color-danger-subtle); }
  .doc pre { margin: 0 0 var(--lds-space-2); padding: var(--lds-space-3); border-radius: var(--lds-radius-md);
    background: var(--lds-color-bg-subtle); overflow-x: auto; font-family: var(--lds-font-family-mono); font-size: var(--lds-font-size-sm); }
  .doc code { font-family: var(--lds-font-family-mono); font-size: 0.9em; padding: 0 0.2em; border-radius: var(--lds-radius-xs);
    background: var(--lds-color-bg-subtle); }
  .doc pre code { padding: 0; background: none; }
  .doc hr { border: 0; border-block-start: 1px solid var(--lds-color-border-default); margin: var(--lds-space-4) 0; }
  .doc img { display: block; max-inline-size: 100%; border-radius: var(--lds-radius-md); margin: 0 0 var(--lds-space-2); }
  .doc table { border-collapse: collapse; margin: 0 0 var(--lds-space-2); }
  .doc td, .doc th { border: 1px solid var(--lds-color-border-default); padding: var(--lds-space-1) var(--lds-space-2); vertical-align: top; }
  .doc th { background: var(--lds-color-bg-subtle); text-align: start; }
  .doc a { color: var(--lds-color-fg-link); }
  .doc .mention, .doc .issue-ref { padding: 0 0.25em; border-radius: var(--lds-radius-xs); background: var(--lds-color-accent-subtle);
    color: var(--lds-color-accent-fg); text-decoration: none; white-space: nowrap; }
`);

/** Emoji by name; unknown names show as :name:. */
export const EMOJI = {
  "+1": "👍", "-1": "👎", tada: "🎉", rocket: "🚀", eyes: "👀", heart: "❤️", fire: "🔥", check: "✅", x: "❌", warning: "⚠️",
  bug: "🐛", sparkles: "✨", thinking: "🤔", smile: "😄", pray: "🙏", hourglass: "⏳", bulb: "💡", memo: "📝", zap: "⚡", lock: "🔒",
};

const MARK_TAGS = { bold: "strong", italic: "em", strike: "s", code: "code" };

/**
 * Renders a document's JSON as escaped HTML inside <div class="doc">.
 * context: { ws, issueKey (for image downloads), people: Map(id → { name }) }.
 */
export function renderDoc(json, context = {}) {
  return html`<div class="doc">${(json?.content ?? []).map((n) => block(n, context))}</div>`;
}

function block(n, ctx) {
  const kids = () => (n.content ?? []).map((c) => block(c, ctx));
  const inline = () => (n.content ?? []).map((c) => inlineNode(c, ctx));
  switch (n.type) {
    case "paragraph":
      return html`<p>${inline()}</p>`;
    case "heading": {
      const level = Math.min(3, Math.max(1, n.attrs?.level ?? 1));
      if (level === 1) return html`<h2>${inline()}</h2>`;
      if (level === 2) return html`<h3>${inline()}</h3>`;
      return html`<h4>${inline()}</h4>`;
    }
    case "bullet_list":
      return html`<ul>${kids()}</ul>`;
    case "ordered_list":
      return html`<ol start="${n.attrs?.start ?? 1}">${kids()}</ol>`;
    case "list_item":
      return html`<li>${kids()}</li>`;
    case "task_list":
      return html`<ul class="tasks">${kids()}</ul>`;
    case "task_item": {
      const checked = !!n.attrs?.checked;
      return html`<li data-checked="${String(checked)}"><input type="checkbox" disabled ${checked ? html`checked` : ""}><div>${kids()}</div></li>`;
    }
    case "quote":
      return html`<blockquote>${kids()}</blockquote>`;
    case "callout":
      return html`<div class="callout" data-tone="${n.attrs?.tone ?? "info"}">${kids()}</div>`;
    case "code_block":
      return html`<pre data-language="${n.attrs?.language ?? ""}"><code>${(n.content ?? []).map((c) => c.text ?? "").join("")}</code></pre>`;
    case "divider":
      return html`<hr>`;
    case "image": {
      const id = n.attrs?.attachment;
      const src = ctx.ws && ctx.issueKey ? `/api/v1/w/${encodeURIComponent(ctx.ws)}/issues/${encodeURIComponent(ctx.issueKey)}/attachments/${id}/download` : "";
      return src ? html`<img src="${src}" alt="${n.attrs?.alt ?? ""}" loading="lazy">` : "";
    }
    case "table":
      return html`<table><tbody>${kids()}</tbody></table>`;
    case "table_row":
      return html`<tr>${kids()}</tr>`;
    case "table_cell":
      return n.attrs?.header ? html`<th>${kids()}</th>` : html`<td>${kids()}</td>`;
  }
  return "";
}

function inlineNode(n, ctx) {
  let out;
  switch (n.type) {
    case "text":
      out = html`${n.text}`;
      break;
    case "hard_break":
      out = html`<br>`;
      break;
    case "mention": {
      const name = ctx.people?.get(n.attrs?.user)?.name ?? "someone";
      out = html`<span class="mention">@${name}</span>`;
      break;
    }
    case "issue_ref": {
      const key = n.attrs?.key ?? "";
      out = ctx.ws ? html`<a class="issue-ref" href="/w/${encodeURIComponent(ctx.ws)}/issues/${encodeURIComponent(key)}">${key}</a>` : html`${key}`;
      break;
    }
    case "emoji":
      out = html`${EMOJI[n.attrs?.name] ?? `:${n.attrs?.name}:`}`;
      break;
    default:
      return "";
  }
  // Marks wrap from the outermost (lowest rank) in.
  for (const m of [...(n.marks ?? [])].reverse()) {
    if (m.type === "link") out = html`<a href="${safeHref(m.attrs?.href)}" target="_blank" rel="noopener noreferrer">${out}</a>`;
    else if (MARK_TAGS[m.type]) out = wrap(MARK_TAGS[m.type], out);
  }
  return out;
}

function wrap(tag, inner) {
  switch (tag) {
    case "strong":
      return html`<strong>${inner}</strong>`;
    case "em":
      return html`<em>${inner}</em>`;
    case "s":
      return html`<s>${inner}</s>`;
    default:
      return html`<code>${inner}</code>`;
  }
}

/** Only http, https and mailto links. */
export function safeHref(href) {
  return /^(https?:|mailto:)/i.test(String(href ?? "")) ? href : "#";
}

/** Whether a document is plain paragraphs of unmarked text. */
export function isPlain(json) {
  return (json?.content ?? []).every((b) => b.type === "paragraph" && (b.content ?? []).every((c) => (c.type === "text" && !c.marks?.length) || c.type === "hard_break"));
}

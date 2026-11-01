// Pasting: HTML (from other apps and from the editor itself) and Markdown
// become document nodes of the schema; anything else is left out. The
// result is a list of blocks, or of inline nodes when it is all one
// paragraph.

import { Fragment, schema } from "horizon/editor/model.js";

const BLOCK_TAGS = new Set(["P", "DIV", "H1", "H2", "H3", "H4", "H5", "H6", "UL", "OL", "LI", "BLOCKQUOTE", "PRE", "HR", "TABLE", "TBODY",
  "THEAD", "TR", "TD", "TH", "FIGURE", "SECTION", "ARTICLE", "HEADER", "FOOTER", "MAIN", "ASIDE"]);
const MARK_OF_TAG = { STRONG: "bold", B: "bold", EM: "italic", I: "italic", S: "strike", DEL: "strike", STRIKE: "strike", CODE: "code" };

const p = (inline = []) => schema.nodes.paragraph.create(null, inline);

/** Text node, merging marks; null for empty text. */
function text(str, marks) {
  return str ? schema.text(str, marks) : null;
}

/** Parses pasted HTML into blocks. */
export function fromHTML(htmlText) {
  const parsed = new DOMParser().parseFromString(htmlText, "text/html");
  return blocksOf(parsed.body);
}

/** The blocks of a DOM element's children; loose inline content becomes paragraphs. */
function blocksOf(el, marks = []) {
  const blocks = [];
  let inline = [];
  // Loose inline content makes a paragraph, unless it is only the
  // whitespace between blocks.
  const flush = () => {
    const content = trimInline(inline);
    if (content.length) blocks.push(p(content));
    inline = [];
  };
  for (const child of el.childNodes) {
    if (child.nodeType === Node.ELEMENT_NODE && BLOCK_TAGS.has(child.tagName)) {
      flush();
      blocks.push(...blockOf(child, marks));
    } else if (child.nodeType === Node.ELEMENT_NODE && child.tagName === "BR" && child.classList.contains("filler")) {
      continue;
    } else {
      inline.push(...inlineOf(child, marks));
    }
  }
  flush();
  return blocks.filter(Boolean);
}

function blockOf(el, marks) {
  const n = schema.nodes;
  switch (el.tagName) {
    case "P":
      return [p(trimInline(inlineChildren(el, marks)))];
    case "H1":
    case "H2":
    case "H3":
    case "H4":
    case "H5":
    case "H6": {
      const level = Math.min(3, Number(el.tagName[1]));
      return [n.heading.create({ level }, trimInline(inlineChildren(el, marks)))];
    }
    case "UL":
    case "OL": {
      const task = el.classList.contains("tasks") || el.querySelector(":scope > li > input[type=checkbox], :scope > li > .check");
      const items = [...el.children].filter((c) => c.tagName === "LI").map((li) => itemOf(li, marks, task));
      if (!items.length) return [];
      if (task) return [n.task_list.create(null, items)];
      if (el.tagName === "OL") return [n.ordered_list.create({ start: Math.max(1, Number(el.getAttribute("start")) || 1) }, items)];
      return [n.bullet_list.create(null, items)];
    }
    case "LI":
      return [n.bullet_list.create(null, [itemOf(el, marks, false)])];
    case "BLOCKQUOTE": {
      const inner = blocksOf(el, marks);
      return [n.quote.create(null, inner.length ? inner : [p()])];
    }
    case "PRE": {
      const code = el.textContent.replace(/\n$/, "");
      return [n.code_block.create({ language: el.dataset?.language ?? "" }, code ? [schema.text(code)] : [])];
    }
    case "HR":
      return [n.divider.create()];
    case "TABLE": {
      const rows = [...el.querySelectorAll("tr")].filter((tr) => tr.closest("table") === el).map((tr) => {
        const cells = [...tr.children].filter((c) => c.tagName === "TD" || c.tagName === "TH").map((cell) => {
          const inner = blocksOf(cell, marks).filter((b) => b.type.name === "paragraph");
          return n.table_cell.create({ header: cell.tagName === "TH" }, inner.length ? inner : [p()]);
        });
        return cells.length ? n.table_row.create(null, cells) : null;
      }).filter(Boolean);
      return rows.length ? [n.table.create(null, rows)] : [];
    }
    case "FIGURE":
      return blocksOf(el, marks);
    default:
      return blocksOf(el, marks);
  }
}

function itemOf(li, marks, task) {
  const n = schema.nodes;
  const box = li.querySelector(":scope > input[type=checkbox]");
  const checked = !!box?.checked || li.dataset?.checked === "true";
  const holder = li.querySelector(":scope > div") && li.querySelector(":scope > .check") ? li.querySelector(":scope > div") : li;
  let inner = blocksOf(holder, marks);
  if (!inner.length || inner[0].type.name !== "paragraph") inner = [p(), ...inner];
  // Nested lists of another kind (bullet in a task list…) stay as they are.
  return task ? n.task_item.create({ checked }, inner) : n.list_item.create(null, inner);
}

function inlineChildren(el, marks) {
  const out = [];
  for (const c of el.childNodes) out.push(...inlineOf(c, marks));
  return out;
}

function inlineOf(node, marks) {
  if (node.nodeType === Node.TEXT_NODE) {
    const t = text(node.data.replace(/\s+/g, " "), marks);
    return t ? [t] : [];
  }
  if (node.nodeType !== Node.ELEMENT_NODE) return [];
  const el = node;
  if (el.tagName === "BR") return el.classList.contains("filler") ? [] : [schema.nodes.hard_break.create()];
  if (el.tagName === "IMG" || el.tagName === "SCRIPT" || el.tagName === "STYLE") return [];
  if (el.classList.contains("issue-ref") && /^[A-Z][A-Z0-9]{1,9}-[1-9][0-9]*$/.test(el.textContent.trim())) {
    return [schema.nodes.issue_ref.create({ key: el.textContent.trim() }, null, marks)];
  }
  let next = marks;
  const markName = MARK_OF_TAG[el.tagName];
  if (markName) next = schema.marks[markName].create().addToSet(next);
  if (el.tagName === "A" && /^(https?:|mailto:)/i.test(el.getAttribute("href") ?? "")) {
    next = schema.marks.link.create({ href: el.getAttribute("href") }).addToSet(next);
  }
  if (el.style?.fontWeight === "bold" || Number(el.style?.fontWeight) >= 600) next = schema.marks.bold.create().addToSet(next);
  if (el.style?.fontStyle === "italic") next = schema.marks.italic.create().addToSet(next);
  return inlineChildren(el, next);
}

/** Drops leading and trailing spaces of a block's inline content. */
function trimInline(nodes) {
  const out = [...nodes];
  while (out.length && out[0].isText && !out[0].text.trim()) out.shift();
  while (out.length && out.at(-1).isText && !out.at(-1).text.trim()) out.pop();
  if (out[0]?.isText) out[0] = out[0].withText(out[0].text.replace(/^\s+/, ""));
  const last = out.at(-1);
  if (last?.isText) out[out.length - 1] = last.withText(last.text.replace(/\s+$/, ""));
  return out.filter((n) => !n.isText || n.text);
}

// --- Markdown ----------------------------------------------------------------------------

/** Whether pasted text looks like Markdown worth parsing. */
export function looksLikeMarkdown(textValue) {
  return /^(#{1,6} |\s*[-*+] |\s*\d+[.)] |> |```|\s*- \[[ xX]\] |---\s*$)/m.test(textValue) || /\*\*[^*]+\*\*|`[^`]+`|\[[^\]]+\]\([^)]+\)/.test(textValue);
}

/** Parses Markdown into blocks: headings, lists (nested by indent), tasks, quotes, fences, rules, paragraphs. */
export function fromMarkdown(md) {
  const lines = md.replace(/\r\n?/g, "\n").split("\n");
  return parseBlocks(lines);
}

function parseBlocks(lines) {
  const n = schema.nodes;
  const blocks = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (!line.trim()) {
      i++;
      continue;
    }
    let m;
    if ((m = /^```(\S*)\s*$/.exec(line))) {
      const body = [];
      i++;
      while (i < lines.length && !/^```\s*$/.test(lines[i])) body.push(lines[i++]);
      i++;
      const code = body.join("\n");
      blocks.push(n.code_block.create({ language: m[1] }, code ? [schema.text(code)] : []));
      continue;
    }
    if (/^(-{3,}|\*{3,}|_{3,})\s*$/.test(line)) {
      blocks.push(n.divider.create());
      i++;
      continue;
    }
    if ((m = /^(#{1,6})\s+(.*)$/.exec(line))) {
      blocks.push(n.heading.create({ level: Math.min(3, m[1].length) }, inlineMarkdown(m[2])));
      i++;
      continue;
    }
    if (/^>\s?/.test(line)) {
      const body = [];
      while (i < lines.length && /^>\s?/.test(lines[i])) body.push(lines[i++].replace(/^>\s?/, ""));
      const inner = parseBlocks(body);
      blocks.push(n.quote.create(null, inner.length ? inner : [p()]));
      continue;
    }
    if (/^\s*([-*+]|\d+[.)])\s+/.test(line)) {
      const { node, next } = parseList(lines, i);
      blocks.push(node);
      i = next;
      continue;
    }
    // A paragraph: lines up to a blank one or another block.
    const body = [line];
    i++;
    while (i < lines.length && lines[i].trim() && !/^(```|#{1,6}\s|>|\s*([-*+]|\d+[.)])\s+|(-{3,}|\*{3,})\s*$)/.test(lines[i])) body.push(lines[i++]);
    const inline = [];
    body.forEach((l, k) => {
      if (k > 0) inline.push(n.hard_break.create());
      inline.push(...inlineMarkdown(l));
    });
    blocks.push(p(inline));
  }
  return blocks;
}

const indentOf = (l) => /^\s*/.exec(l)[0].replace(/\t/g, "  ").length;

function parseList(lines, start) {
  const n = schema.nodes;
  const base = indentOf(lines[start]);
  const ordered = /^\s*\d+[.)]\s/.test(lines[start]);
  const task = /^\s*[-*+]\s+\[[ xX]\]\s/.test(lines[start]);
  const items = [];
  let i = start;
  while (i < lines.length && lines[i].trim() && indentOf(lines[i]) === base && /^\s*([-*+]|\d+[.)])\s+/.test(lines[i])) {
    let content = lines[i].replace(/^\s*([-*+]|\d+[.)])\s+/, "");
    let checked = false;
    const tm = /^\[([ xX])\]\s+(.*)$/.exec(content);
    if (task && tm) {
      checked = tm[1] !== " ";
      content = tm[2];
    }
    i++;
    // Lines indented deeper belong to this item.
    const nested = [];
    while (i < lines.length && lines[i].trim() && indentOf(lines[i]) > base) nested.push(lines[i++].slice(base + 2));
    const inner = [p(inlineMarkdown(content)), ...parseBlocks(nested)];
    items.push(task ? n.task_item.create({ checked }, inner) : n.list_item.create(null, inner));
  }
  const startNumber = ordered ? Number(/\d+/.exec(lines[start])[0]) || 1 : 1;
  const node = task ? n.task_list.create(null, items) : ordered ? n.ordered_list.create({ start: startNumber }, items) : n.bullet_list.create(null, items);
  return { node, next: i };
}

/** Inline Markdown: **bold**, *italic* or _italic_, ~~strike~~, `code`, [text](url). */
export function inlineMarkdown(src, marks = []) {
  const out = [];
  const pattern = /(\*\*|__)(.+?)\1|(\*|_)(.+?)\3|~~(.+?)~~|`([^`]+)`|\[([^\]]+)\]\(([^)\s]+)\)/g;
  let last = 0;
  let m;
  while ((m = pattern.exec(src))) {
    if (m.index > last) pushText(out, src.slice(last, m.index), marks);
    if (m[2] !== undefined) out.push(...inlineMarkdown(m[2], schema.marks.bold.create().addToSet(marks)));
    else if (m[4] !== undefined) out.push(...inlineMarkdown(m[4], schema.marks.italic.create().addToSet(marks)));
    else if (m[5] !== undefined) out.push(...inlineMarkdown(m[5], schema.marks.strike.create().addToSet(marks)));
    else if (m[6] !== undefined) pushText(out, m[6], schema.marks.code.create().addToSet(marks));
    else if (/^(https?:|mailto:)/i.test(m[8])) out.push(...inlineMarkdown(m[7], schema.marks.link.create({ href: m[8] }).addToSet(marks)));
    else pushText(out, m[0], marks);
    last = pattern.lastIndex;
  }
  if (last < src.length) pushText(out, src.slice(last), marks);
  return Fragment.fromArray(out).content; // adjacent text with the same marks joins
}

function pushText(out, str, marks) {
  const t = text(str, marks);
  if (t) out.push(t);
}

/** Whether blocks are a single paragraph (paste its inline content in place). */
export function inlineOnly(blocks) {
  return blocks.length === 1 && blocks[0].type.name === "paragraph";
}


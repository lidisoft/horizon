// Hand-written SVG charts (docs/reports.md#charts): hzn-chart-line,
// hzn-chart-area (stacked), hzn-chart-bar (grouped, stacked or
// horizontal), hzn-chart-scatter and hzn-sparkline. Colours come from the
// LDS chart tokens by tone (completed, started, unstarted, abandoned,
// reference, annotation, marker). Every chart but the sparkline has a
// legend when it has two series or more, a tooltip on hover, and a table
// view (the button under it), for screen readers and the low-contrast
// unstarted tone.
//
//   chart.data = {
//     labels: ["2026-11-02", …],          // the x axis (line, area, bar)
//     series: [{ name, tone, values: [n|null…], dashed }],
//     marker: index,                      // a vertical marker (today)
//     stacked: true, horizontal: true,    // bar
//     points: [{ x: label, y, name }],    // scatter
//     refs: [{ name, value, tone }],      // scatter: horizontal lines
//     format: (v) => string,              // values (default: a number)
//   }

import { html, sheet } from "horizon/core/dom.js";

const TONES = ["completed", "started", "unstarted", "abandoned", "reference", "annotation", "marker"];
const PAD = { top: 12, right: 12, bottom: 28, left: 40 };

const styles = sheet(`
  :host { display: block; position: relative; }
  figure { margin: 0; display: grid; gap: var(--lds-space-2); }
  svg { display: block; inline-size: 100%; overflow: visible; font-family: inherit; }
  .grid line { stroke: var(--lds-color-border-subtle); stroke-width: 1; }
  .axis text { fill: var(--lds-color-fg-muted); font-size: 11px; }
  .axis .base { stroke: var(--lds-color-border-default); }
  path.line { fill: none; stroke-width: 2; stroke-linejoin: round; stroke-linecap: round; }
  path.line.dashed { stroke-dasharray: 5 4; }
  path.area { stroke: var(--lds-color-bg-surface); stroke-width: 1; }
  rect.bar { stroke: var(--lds-color-bg-surface); stroke-width: var(--lds-chart-segment-gap, 2px); }
  circle.point { stroke: var(--lds-color-bg-surface); stroke-width: 2; }
  line.marker { stroke: var(--lds-color-chart-marker); stroke-width: 2; }
  line.ref { stroke-width: 1.5; stroke-dasharray: 5 4; }
  line.cross { stroke: var(--lds-color-border-strong); stroke-width: 1; }
  text.ref-label { font-size: 11px; fill: var(--lds-color-fg-muted); }
  ${TONES.map((t) => `.t-${t} { stroke: var(--lds-color-chart-${t}); }
  path.area.t-${t}, rect.t-${t}, circle.t-${t}, .swatch.t-${t} { fill: var(--lds-color-chart-${t}); background: var(--lds-color-chart-${t}); }`).join("\n  ")}
  .legend { display: flex; flex-wrap: wrap; gap: var(--lds-space-1) var(--lds-space-4); font-size: var(--lds-font-size-sm); color: var(--lds-color-fg-default); }
  .legend span { display: inline-flex; align-items: center; gap: var(--lds-space-1); }
  .swatch { inline-size: 0.75rem; block-size: 0.75rem; border-radius: 2px; }
  .swatch.dashed { block-size: 2px; border-radius: 0; }
  .foot { display: flex; align-items: center; justify-content: space-between; gap: var(--lds-space-2); }
  .foot lds-button { margin-inline-start: auto; }
  .tip { position: absolute; pointer-events: none; z-index: 2; padding: var(--lds-space-2); border-radius: var(--lds-radius-sm);
    background: var(--lds-color-bg-surface); border: 1px solid var(--lds-color-border-default); box-shadow: var(--lds-shadow-md);
    font-size: var(--lds-font-size-xs); white-space: nowrap; }
  .tip[hidden] { display: none; }
  .tip strong { display: block; margin-block-end: 2px; }
  .tip div { display: flex; align-items: center; gap: var(--lds-space-1); }
  table { border-collapse: collapse; font-size: var(--lds-font-size-sm); }
  th, td { padding: 2px var(--lds-space-2); text-align: end; border-block-end: 1px solid var(--lds-color-border-subtle); font-variant-numeric: tabular-nums; }
  th:first-child, td:first-child { text-align: start; }
  .sr-only { position: absolute; inline-size: 1px; block-size: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
  .empty { color: var(--lds-color-fg-muted); font-size: var(--lds-font-size-sm); padding: var(--lds-space-6) 0; text-align: center; }
`);

const numberFormat = new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 });

/** Round axis ticks from 0 to at least max: 1, 2 or 5 × 10ⁿ apart. */
export function niceTicks(max, count = 4) {
  if (!(max > 0)) return [0, 1];
  const raw = max / count;
  const mag = 10 ** Math.floor(Math.log10(raw));
  const step = [1, 2, 5, 10].map((m) => m * mag).find((s) => s >= raw);
  const ticks = [];
  for (let v = 0; v < max + step / 2; v += step) ticks.push(Number(v.toPrecision(12)));
  if (ticks[ticks.length - 1] < max) ticks.push(Number((ticks[ticks.length - 1] + step).toPrecision(12)));
  return ticks;
}

/** Which of n labels to show in width pixels, about every minGap. */
export function labelStep(n, width, minGap = 56) {
  return Math.max(1, Math.ceil((n * minGap) / Math.max(width, 1)));
}

/** Stacks series: for each series and index, its base and top. */
export function stack(series) {
  const n = Math.max(0, ...series.map((s) => s.values.length));
  const base = new Array(n).fill(0);
  return series.map((s) => s.values.map((v, i) => {
    const b = base[i];
    base[i] += v ?? 0;
    return v == null ? null : [b, b + v];
  }));
}

/** Reference lines, those at the same value merged into one (p50 · p85). */
export function refLines(refs) {
  const out = [];
  for (const r of refs) {
    if (r.value == null) continue;
    const same = out.find((x) => x.value === r.value);
    if (same) same.name += ` · ${r.name}`;
    else out.push({ ...r });
  }
  return out;
}

/** A short label for an ISO date, else the label itself. */
export function shortLabel(label, locale) {
  if (/^\d{4}-\d{2}-\d{2}$/.test(label)) {
    return new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", timeZone: "UTC" }).format(new Date(`${label}T00:00:00Z`));
  }
  return String(label);
}

class Chart extends HTMLElement {
  connectedCallback() {
    if (this.shadowRoot) return;
    const root = this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [styles];
    this.showTable = false;
    root.addEventListener("click", (e) => {
      if (e.target.closest?.("[data-toggle]")) {
        this.showTable = !this.showTable;
        this.render();
      }
    });
    this.observer = new ResizeObserver(() => {
      if (this.clientWidth !== this.width) this.render();
    });
    this.observer.observe(this);
    this.render();
  }

  disconnectedCallback() {
    this.observer?.disconnect();
  }

  set data(v) {
    this._data = v;
    if (this.shadowRoot) this.render();
  }

  get data() {
    return this._data;
  }

  get height() {
    return Number(this.getAttribute("height")) || 240;
  }

  fmt(v) {
    if (v == null) return "–";
    return this._data?.format ? this._data.format(v) : numberFormat.format(v);
  }

  /** The table view: a row per label, a column per series. */
  table() {
    const d = this._data;
    return html`<table><caption class="sr-only">${this.getAttribute("label") ?? ""}</caption>
      <thead><tr><th></th>${d.series.map((s) => html`<th scope="col">${s.name}</th>`)}</tr></thead>
      <tbody>${d.labels.map((l, i) => html`<tr><th scope="row">${l}</th>${d.series.map((s) => html`<td>${this.fmt(s.values[i])}</td>`)}</tr>`)}</tbody>
    </table>`;
  }

  legend() {
    const series = this._data.series ?? [];
    if (series.length < 2) return "";
    return html`<div class="legend">${series.map((s) => html`<span><i class="swatch t-${s.tone}${s.dashed ? " dashed" : ""}"></i>${s.name}</span>`)}</div>`;
  }

  render() {
    const d = this._data;
    this.width = this.clientWidth;
    const label = this.getAttribute("label") ?? "";
    if (!d || (!d.series?.some((s) => s.values.some((v) => v != null)) && !d.points?.length)) {
      this.shadowRoot.innerHTML = String(html`<p class="empty">${this.getAttribute("empty") ?? ""}</p>`);
      return;
    }
    const body = this.showTable ? this.table() : html`<svg role="img" aria-label="${label}" viewBox="0 0 ${Math.max(this.width, 1)} ${this.height}"
      width="${Math.max(this.width, 1)}" height="${this.height}">${this.draw(Math.max(this.width, 1), this.height)}</svg>`;
    this.shadowRoot.innerHTML = String(html`
      <figure>
        ${body}
        <div class="foot">${this.legend()}
          <lds-button variant="ghost" size="sm" data-toggle>${this.showTable ? (this.getAttribute("chart-label") ?? "Chart") : (this.getAttribute("table-label") ?? "Table")}</lds-button>
        </div>
      </figure>
      <div class="tip" hidden></div>`);
    const svg = this.shadowRoot.querySelector("svg");
    if (svg) {
      svg.addEventListener("pointermove", (e) => this.hover(e, svg));
      svg.addEventListener("pointerleave", () => this.unhover());
    }
  }

  /** The y scale over values up to max. */
  yScale(max, h) {
    const ticks = niceTicks(max);
    const top = ticks[ticks.length - 1];
    const inner = h - PAD.top - PAD.bottom;
    return { ticks, y: (v) => PAD.top + inner - (v / top) * inner };
  }

  /** Grid lines, y ticks and x labels. */
  axes(w, h, ys, xs) {
    const d = this._data;
    const step = labelStep(d.labels.length, w - PAD.left - PAD.right);
    return html`
      <g class="grid">${ys.ticks.map((v) => html`<line x1="${PAD.left}" x2="${w - PAD.right}" y1="${ys.y(v)}" y2="${ys.y(v)}"></line>`)}</g>
      <g class="axis">
        ${ys.ticks.map((v) => html`<text x="${PAD.left - 6}" y="${ys.y(v) + 4}" text-anchor="end">${this.fmt(v)}</text>`)}
        <line class="base" x1="${PAD.left}" x2="${w - PAD.right}" y1="${h - PAD.bottom}" y2="${h - PAD.bottom}"></line>
        ${d.labels.map((l, i) => (i % step === 0 ? html`<text x="${xs(i)}" y="${h - PAD.bottom + 16}" text-anchor="middle">${shortLabel(l)}</text>` : ""))}
      </g>
      ${d.marker != null && d.marker >= 0 && d.marker < d.labels.length ? html`<line class="marker" x1="${xs(d.marker)}" x2="${xs(d.marker)}" y1="${PAD.top}" y2="${h - PAD.bottom}"></line>` : ""}`;
  }

  /** The x position of index i on a point axis (lines) or band axis (bars). */
  xPoint(w, n) {
    const inner = w - PAD.left - PAD.right;
    return (i) => PAD.left + (n <= 1 ? inner / 2 : (i * inner) / (n - 1));
  }

  /** Index under the pointer, for x-based charts. */
  indexAt(e, svg) {
    const n = this._data.labels.length;
    const box = svg.getBoundingClientRect();
    const x = ((e.clientX - box.left) / box.width) * this.width;
    const inner = this.width - PAD.left - PAD.right;
    return Math.max(0, Math.min(n - 1, Math.round(((x - PAD.left) / inner) * (n - 1))));
  }

  hover(e, svg) {
    const d = this._data;
    if (!d.labels) return;
    const i = this.indexAt(e, svg);
    const tip = this.shadowRoot.querySelector(".tip");
    tip.innerHTML = String(html`<strong>${d.labels[i]}</strong>${d.series.map((s) => html`<div><i class="swatch t-${s.tone}"></i>${s.name}: ${this.fmt(s.values[i])}</div>`)}`);
    tip.hidden = false;
    const box = this.getBoundingClientRect();
    const left = Math.min(e.clientX - box.left + 12, box.width - tip.offsetWidth - 4);
    tip.style.setProperty("left", `${Math.max(0, left)}px`);
    tip.style.setProperty("top", `${PAD.top}px`);
    let cross = svg.querySelector("line.cross");
    if (!cross) {
      cross = document.createElementNS("http://www.w3.org/2000/svg", "line");
      cross.setAttribute("class", "cross");
      svg.append(cross);
    }
    const x = this.crossX(i);
    for (const [k, v] of [["x1", x], ["x2", x], ["y1", PAD.top], ["y2", this.height - PAD.bottom]]) cross.setAttribute(k, v);
  }

  crossX(i) {
    return this.xPoint(this.width, this._data.labels.length)(i);
  }

  unhover() {
    const tip = this.shadowRoot.querySelector(".tip");
    if (tip) tip.hidden = true;
    this.shadowRoot.querySelector("line.cross")?.remove();
  }

  /** A path through the defined points of values, broken at nulls. */
  static path(values, xs, y) {
    let d = "";
    let pen = false;
    values.forEach((v, i) => {
      if (v == null) {
        pen = false;
        return;
      }
      d += `${pen ? "L" : "M"}${xs(i).toFixed(1)} ${y(v).toFixed(1)}`;
      pen = true;
    });
    return d;
  }
}

class LineChart extends Chart {
  draw(w, h) {
    const d = this._data;
    const max = Math.max(0, ...d.series.flatMap((s) => s.values.filter((v) => v != null)));
    const ys = this.yScale(max, h);
    const xs = this.xPoint(w, d.labels.length);
    // A value with no neighbour has no segment to draw: it is a dot.
    const lone = (values, i) => values[i] != null && values[i - 1] == null && values[i + 1] == null;
    return html`${this.axes(w, h, ys, xs)}
      ${d.series.map((s) => html`<path class="line t-${s.tone}${s.dashed ? " dashed" : ""}" d="${Chart.path(s.values, xs, ys.y)}"></path>
        ${s.values.map((v, i) => (lone(s.values, i) ? html`<circle class="point t-${s.tone}" cx="${xs(i)}" cy="${ys.y(v)}" r="4"></circle>` : ""))}`)}`;
  }
}

class AreaChart extends Chart {
  draw(w, h) {
    const d = this._data;
    const stacks = stack(d.series);
    const max = Math.max(0, ...stacks.flatMap((s) => s.filter(Boolean).map((p) => p[1])));
    const ys = this.yScale(max, h);
    const xs = this.xPoint(w, d.labels.length);
    const areas = d.series.map((s, k) => {
      const pts = stacks[k].map((p, i) => (p ? { i, p } : null)).filter(Boolean);
      if (!pts.length) return "";
      const top = pts.map(({ i, p }) => `${xs(i).toFixed(1)} ${ys.y(p[1]).toFixed(1)}`).join(" L");
      const bottom = [...pts].reverse().map(({ i, p }) => `${xs(i).toFixed(1)} ${ys.y(p[0]).toFixed(1)}`).join(" L");
      return html`<path class="area t-${s.tone}" d="M${top} L${bottom} Z"></path>`;
    });
    return html`${this.axes(w, h, ys, xs)}${areas}`;
  }
}

class BarChart extends Chart {
  get horizontal() {
    return !!this._data?.horizontal;
  }

  get height() {
    if (this.horizontal) return Math.max(80, this._data.labels.length * 28 + PAD.top + PAD.bottom);
    return super.height;
  }

  band(w) {
    const n = this._data.labels.length;
    const inner = w - PAD.left - PAD.right;
    const bw = inner / Math.max(n, 1);
    return { bw, x: (i) => PAD.left + i * bw };
  }

  crossX(i) {
    const { bw, x } = this.band(this.width);
    return x(i) + bw / 2;
  }

  indexAt(e, svg) {
    const n = this._data.labels.length;
    const box = svg.getBoundingClientRect();
    if (this.horizontal) {
      const y = ((e.clientY - box.top) / box.height) * this.height;
      return Math.max(0, Math.min(n - 1, Math.floor((y - PAD.top) / 28)));
    }
    const x = ((e.clientX - box.left) / box.width) * this.width;
    const { bw } = this.band(this.width);
    return Math.max(0, Math.min(n - 1, Math.floor((x - PAD.left) / bw)));
  }

  draw(w, h) {
    const d = this._data;
    const stacked = !!d.stacked;
    const tops = stacked ? stack(d.series) : d.series.map((s) => s.values.map((v) => (v == null ? null : [0, v])));
    const max = Math.max(0, ...tops.flatMap((s) => s.filter(Boolean).map((p) => p[1])));
    if (this.horizontal) return this.drawHorizontal(w, h, tops, max);
    const ys = this.yScale(max, h);
    const { bw, x } = this.band(w);
    const groups = stacked ? 1 : d.series.length;
    const gap = Math.min(8, bw * 0.2);
    const each = Math.max(1, (bw - gap * 2) / groups);
    const xs = (i) => x(i) + bw / 2;
    const bars = d.series.map((s, k) => tops[k].map((p, i) => {
      if (!p || p[1] === p[0]) return "";
      const bx = x(i) + gap + (stacked ? 0 : k * each);
      return html`<rect class="bar t-${s.tone}" x="${bx}" width="${each}" y="${ys.y(p[1])}" height="${Math.max(0, ys.y(p[0]) - ys.y(p[1]))}"
        rx="${stacked ? 0 : 3}"></rect>`;
    }));
    return html`${this.axes(w, h, ys, xs)}${bars}`;
  }

  drawHorizontal(w, h, tops, max) {
    const d = this._data;
    const left = Math.min(180, w * 0.35);
    const inner = w - left - PAD.right - 40;
    const ticks = niceTicks(max);
    const top = ticks[ticks.length - 1];
    const xv = (v) => left + (v / top) * inner;
    return html`
      <g class="grid">${ticks.map((v) => html`<line x1="${xv(v)}" x2="${xv(v)}" y1="${PAD.top}" y2="${h - PAD.bottom}"></line>`)}</g>
      <g class="axis">${ticks.map((v) => html`<text x="${xv(v)}" y="${h - PAD.bottom + 16}" text-anchor="middle">${this.fmt(v)}</text>`)}
        ${d.labels.map((l, i) => html`<text x="${left - 8}" y="${PAD.top + i * 28 + 18}" text-anchor="end">${String(l).slice(0, 28)}</text>`)}</g>
      ${d.series.map((s, k) => tops[k].map((p, i) => (p ? html`
        <rect class="bar t-${s.tone}" x="${xv(p[0])}" width="${Math.max(0, xv(p[1]) - xv(p[0]))}" y="${PAD.top + i * 28 + 6}" height="16" rx="3"></rect>
        ${k === d.series.length - 1 ? html`<text class="ref-label" x="${xv(p[1]) + 6}" y="${PAD.top + i * 28 + 18}">${this.fmt(p[1])}</text>` : ""}` : "")))}`;
  }
}

class ScatterChart extends Chart {
  legend() {
    const refs = this._data.refs ?? [];
    if (!refs.length) return "";
    return html`<div class="legend">${refs.map((r) => html`<span><i class="swatch dashed t-${r.tone}"></i>${r.name}: ${this.fmt(r.value)}</span>`)}</div>`;
  }

  table() {
    const d = this._data;
    return html`<table><thead><tr><th></th><th scope="col">${d.xName ?? ""}</th><th scope="col">${d.yName ?? ""}</th></tr></thead>
      <tbody>${d.points.map((p) => html`<tr><th scope="row">${p.name}</th><td>${p.x}</td><td>${this.fmt(p.y)}</td></tr>`)}</tbody></table>`;
  }

  draw(w, h) {
    const d = this._data;
    const labels = [...new Set(d.points.map((p) => p.x))].sort();
    this._labels = labels;
    const max = Math.max(0, ...d.points.map((p) => p.y), ...(d.refs ?? []).map((r) => r.value ?? 0));
    const ys = this.yScale(max, h);
    const xs = this.xPoint(w, labels.length);
    const at = new Map(labels.map((l, i) => [l, i]));
    const saved = this._data.labels;
    this._data.labels = labels;
    const axes = this.axes(w, h, ys, xs);
    this._data.labels = saved;
    return html`${axes}
      ${refLines(d.refs ?? []).map((r) => html`<line class="ref t-${r.tone}" x1="${PAD.left}" x2="${w - PAD.right}" y1="${ys.y(r.value)}" y2="${ys.y(r.value)}"></line>
        <text class="ref-label" x="${w - PAD.right}" y="${ys.y(r.value) - 4}" text-anchor="end">${r.name}</text>`)}
      ${d.points.map((p) => html`<circle class="point t-${p.tone ?? "completed"}" cx="${xs(at.get(p.x))}" cy="${ys.y(p.y)}" r="5"></circle>`)}`;
  }

  hover(e, svg) {
    const d = this._data;
    const box = svg.getBoundingClientRect();
    const mx = ((e.clientX - box.left) / box.width) * this.width;
    const my = ((e.clientY - box.top) / box.height) * this.height;
    let best = null;
    for (const c of svg.querySelectorAll("circle.point")) {
      const dx = Number(c.getAttribute("cx")) - mx;
      const dy = Number(c.getAttribute("cy")) - my;
      const dist = dx * dx + dy * dy;
      if (!best || dist < best.dist) best = { c, dist };
    }
    const tip = this.shadowRoot.querySelector(".tip");
    if (!best || best.dist > 400) {
      tip.hidden = true;
      return;
    }
    const p = d.points[[...svg.querySelectorAll("circle.point")].indexOf(best.c)];
    tip.innerHTML = String(html`<strong>${p.name}</strong><div>${p.x}: ${this.fmt(p.y)}</div>`);
    tip.hidden = false;
    tip.style.setProperty("left", `${Math.min(mx + 12, this.width - tip.offsetWidth - 4)}px`);
    tip.style.setProperty("top", `${Math.max(0, my - 40)}px`);
  }
}

/** A small line with no axes, legend or table: its label says what it shows. */
class Sparkline extends HTMLElement {
  connectedCallback() {
    if (!this.shadowRoot) this.attachShadow({ mode: "open" }).adoptedStyleSheets = [styles];
    this.render();
  }

  set data(v) {
    this._data = v;
    if (this.shadowRoot) this.render();
  }

  render() {
    const d = this._data;
    const w = Number(this.getAttribute("width")) || 120;
    const h = Number(this.getAttribute("height")) || 32;
    const values = d?.values ?? [];
    const max = Math.max(1, ...values.filter((v) => v != null), ...(d?.ref ?? []).filter((v) => v != null));
    const xs = (i) => 2 + (values.length <= 1 ? 0 : (i * (w - 4)) / (values.length - 1));
    const y = (v) => h - 2 - (v / max) * (h - 4);
    this.shadowRoot.innerHTML = String(html`<svg role="img" aria-label="${this.getAttribute("label") ?? ""}" viewBox="0 0 ${w} ${h}" width="${w}" height="${h}">
      ${d?.ref ? html`<path class="line dashed t-reference" d="${Chart.path(d.ref, xs, y)}"></path>` : ""}
      <path class="line t-${d?.tone ?? "started"}" d="${Chart.path(values, xs, y)}"></path></svg>`);
  }
}

customElements.define("hzn-chart-line", LineChart);
customElements.define("hzn-chart-area", AreaChart);
customElements.define("hzn-chart-bar", BarChart);
customElements.define("hzn-chart-scatter", ScatterChart);
customElements.define("hzn-sparkline", Sparkline);

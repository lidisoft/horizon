// Static viewer for the Horizon wireframes.
//
// The boards are authored on a Claude Design canvas as .dc.html files. Their
// templates and logic are copied verbatim into boards.js; this file renders
// them with the small subset of the canvas template language they use:
// {{ path }} interpolation, <sc-for list as>, <sc-if value>, <dc-import name>
// and <helmet>. It needs no server: open index.html from disk.
"use strict";

(() => {
  const BOARDS = window.HZ_BOARDS;
  const META = window.HZ_META;
  const app = document.getElementById("app");

  // ------------------------------------------------------------------------
  // Template parsing. The source is well-formed HTML written by hand, so a
  // tag tokenizer is enough. It works on the raw text because the browser's
  // own parser would move <sc-for> out of <tbody>.
  // ------------------------------------------------------------------------

  const VOID = new Set(["area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "source", "track", "wbr"]);
  const TAG = /<!--[\s\S]*?-->|<\/([a-zA-Z][\w:-]*)\s*>|<([a-zA-Z][\w:-]*)((?:\s+[^\s"'>/=]+(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s"'=<>`]+))?)*)\s*(\/?)>/g;
  const ATTR = /([^\s"'>/=]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>`]+)))?/g;

  function parseAttrs(source) {
    const attrs = [];
    for (const m of source.matchAll(ATTR)) {
      const value = m[2] ?? m[3] ?? m[4];
      attrs.push({ name: m[1], value: value === undefined ? null : value });
    }
    return attrs;
  }

  function parse(src) {
    const root = { tag: "#root", children: [] };
    const stack = [root];
    const top = () => stack[stack.length - 1];
    let last = 0;
    TAG.lastIndex = 0;
    let m;
    while ((m = TAG.exec(src))) {
      if (m.index > last) top().children.push({ text: src.slice(last, m.index) });
      last = TAG.lastIndex;
      if (m[0].startsWith("<!--")) continue;
      if (m[1]) {
        const tag = m[1].toLowerCase();
        for (let k = stack.length - 1; k > 0; k--) {
          if (stack[k].tag === tag) { stack.length = k; break; }
        }
        continue;
      }
      const el = { tag: m[2].toLowerCase(), name: m[2], attrs: parseAttrs(m[3]), children: [] };
      top().children.push(el);
      if (el.tag === "style" || el.tag === "script") {
        const end = src.indexOf("</" + el.tag, last);
        el.children.push({ text: src.slice(last, end) });
        last = src.indexOf(">", end) + 1;
        TAG.lastIndex = last;
        continue;
      }
      if (!VOID.has(el.tag) && !m[4]) stack.push(el);
    }
    if (last < src.length) top().children.push({ text: src.slice(last) });
    return root;
  }

  // ------------------------------------------------------------------------
  // Expressions and rendering
  // ------------------------------------------------------------------------

  const fns = new Map();
  function evaluate(expr, scope) {
    let fn = fns.get(expr);
    if (!fn) {
      // Sloppy-mode Function so `with` resolves names against the scope.
      fn = new Function("$scope", "with ($scope) { return (" + expr + "); }");
      fns.set(expr, fn);
    }
    try {
      return fn(scope);
    } catch (err) {
      // Names a template uses before its data defines them read as empty.
      if (window.HZ_DEBUG) console.warn("wireframes: {{ " + expr + " }}", err.message);
      return undefined;
    }
  }

  const escText = (s) => String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  const escAttr = (s) => String(s).replace(/&/g, "&amp;").replace(/"/g, "&quot;");
  const EXPR = /\{\{\s*([\s\S]+?)\s*\}\}/g;
  const ONLY_EXPR = /^\s*\{\{\s*([\s\S]+?)\s*\}\}\s*$/;

  function interpolate(str, scope, esc) {
    return str.replace(EXPR, (_, e) => {
      const v = evaluate(e, scope);
      return v == null ? "" : esc(String(v));
    });
  }

  // A value that is a single {{ expr }} keeps its type (lists, booleans).
  function value(str, scope) {
    if (str == null) return true;
    const m = ONLY_EXPR.exec(str);
    return m ? evaluate(m[1], scope) : interpolate(str, scope, (s) => s);
  }

  const attr = (el, name) => (el.attrs.find((a) => a.name === name) || {}).value;

  function renderChildren(node, scope, ctx) {
    let out = "";
    for (const child of node.children) out += renderNode(child, scope, ctx);
    return out;
  }

  function renderNode(node, scope, ctx) {
    if ("text" in node) return interpolate(node.text, scope, escText);
    switch (node.tag) {
      case "sc-for": {
        const list = value(attr(node, "list"), scope) || [];
        const as = attr(node, "as") || "item";
        let out = "";
        for (const item of list) {
          const inner = Object.create(scope);
          inner[as] = item;
          out += renderChildren(node, inner, ctx);
        }
        return out;
      }
      case "sc-if":
        return value(attr(node, "value"), scope) ? renderChildren(node, scope, ctx) : "";
      case "helmet":
        ctx.helmet.add(renderChildren(node, scope, ctx));
        return "";
      case "dc-import": {
        const props = {};
        for (const a of node.attrs) {
          if (a.name !== "name" && !a.name.startsWith("hint-")) props[a.name] = value(a.value, scope);
        }
        return renderBoard(attr(node, "name"), props, ctx);
      }
      case "style":
      case "script":
        return "<" + node.name + ">" + node.children[0].text + "</" + node.name + ">";
    }
    let open = "<" + node.name;
    for (const a of node.attrs) {
      if (a.value === null) { open += " " + a.name; continue; }
      let v = interpolate(a.value, scope, (s) => s);
      const link = a.name === "href" && /^([A-Za-z]\w*)\.dc\.html$/.exec(v);
      if (link) v = "#/" + link[1];
      open += " " + a.name + '="' + escAttr(v) + '"';
    }
    open += ">";
    if (VOID.has(node.tag)) return open;
    return open + renderChildren(node, scope, ctx) + "</" + node.name + ">";
  }

  class DCLogic {
    constructor(props) { this.props = props || {}; }
  }

  const compiled = new Map();
  function compile(name) {
    if (!compiled.has(name)) {
      const b = BOARDS[name];
      const Component = new Function("DCLogic", b.script + "\n;return typeof Component === 'undefined' ? null : Component;")(DCLogic);
      compiled.set(name, { tree: parse(b.template), Component });
    }
    return compiled.get(name);
  }

  function renderBoard(name, props, ctx) {
    if (!BOARDS[name]) return "<!-- unknown board " + name + " -->";
    const { tree, Component } = compile(name);
    const vals = Component ? new Component(props).renderVals() : {};
    return renderChildren(tree, Object.assign(Object.create(null), vals), ctx);
  }

  // ------------------------------------------------------------------------
  // Pages
  // ------------------------------------------------------------------------

  function setTheme(theme) {
    document.documentElement.setAttribute("data-theme", theme);
    document.documentElement.setAttribute("data-lds-theme", theme);
  }

  function gallery() {
    setTheme(localTheme());
    document.title = "Horizon wireframes";
    let html = '<div class="hz-gallery"><header class="hz-gallery__head">' +
      '<h1 class="lds-title">Horizon wireframes</h1>' +
      '<p class="lds-muted">' + escText(META.boards) + " boards, LDS " + escText(META.lds) +
      ". Static copy of the Claude Design canvas; open any board to switch its theme and states.</p></header>";
    for (const group of META.groups) {
      html += '<section class="hz-group"><h2 class="hz-group__title">' + escText(group.title) + '</h2><div class="hz-grid">';
      for (const name of group.boards) {
        const b = BOARDS[name];
        const extras = Object.keys(b.props).filter((k) => k !== "theme");
        html += '<a class="hz-card" href="#/' + name + '"><span class="hz-card__title">' + escText(b.title) + "</span>" +
          '<span class="hz-card__meta">' + b.w + " × " + b.h + (extras.length ? " · " + escText(extras.join(", ")) : "") + "</span></a>";
      }
      html += "</div></section>";
    }
    app.innerHTML = html + "</div>";
  }

  function localTheme() {
    try { return localStorage.getItem("hz-wireframes-theme") || "light"; } catch { return "light"; }
  }

  function board(name, params) {
    const b = BOARDS[name];
    if (!b) return gallery();
    const props = {};
    for (const [k, spec] of Object.entries(b.props)) props[k] = params.get(k) ?? spec.default;
    if (!("theme" in props)) props.theme = params.get("theme") ?? localTheme();
    setTheme(props.theme);
    try { localStorage.setItem("hz-wireframes-theme", props.theme); } catch { /* private mode */ }
    document.title = b.title + " · Horizon wireframes";

    const order = META.order;
    const i = order.indexOf(name);
    const prev = order[(i - 1 + order.length) % order.length];
    const next = order[(i + 1) % order.length];

    const controls = Object.entries({ theme: { options: ["light", "dark"] }, ...b.props })
      .filter(([, spec]) => spec.options)
      .map(([k, spec]) =>
        '<label class="hz-control"><span>' + escText(k) + '</span><lds-segmented size="sm" label="' + escAttr(k) + '" data-prop="' + escAttr(k) + '" value="' + escAttr(String(props[k])) + '">' +
        spec.options.map((o) => '<lds-segment value="' + escAttr(o) + '">' + escText(o) + "</lds-segment>").join("") +
        "</lds-segmented></label>")
      .join("");

    const ctx = { helmet: new Set() };
    const body = renderBoard(name, props, ctx);
    app.innerHTML =
      '<div class="hz-toolbar">' +
      '<a class="hz-back" href="#/">All wireframes</a>' +
      '<strong class="hz-toolbar__title">' + escText(b.title) + "</strong>" +
      controls +
      '<span class="hz-spacer"></span>' +
      '<a href="#/' + prev + '" aria-label="Previous board">‹ ' + escText(BOARDS[prev].title) + "</a>" +
      '<a href="#/' + next + '" aria-label="Next board">' + escText(BOARDS[next].title) + " ›</a>" +
      "</div>" +
      '<div class="hz-stage"><div class="hz-frame" style="width: ' + b.w + "px; min-height: " + b.h + 'px;">' +
      [...ctx.helmet].join("") + body + "</div></div>";

    for (const seg of app.querySelectorAll("lds-segmented[data-prop]")) {
      seg.addEventListener("lds-change", (event) => {
        params.set(seg.dataset.prop, event.detail.value);
        location.hash = "#/" + name + "?" + params.toString();
      });
    }
    window.scrollTo(0, 0);
  }

  function route() {
    const hash = location.hash.replace(/^#\/?/, "");
    if (!hash) return gallery();
    const [name, query] = hash.split("?");
    board(decodeURIComponent(name), new URLSearchParams(query || ""));
  }

  // Placeholder links in the boards (href="#") stay on the current board.
  app.addEventListener("click", (event) => {
    const link = event.target.closest && event.target.closest('a[href="#"]');
    if (link) event.preventDefault();
  });
  window.addEventListener("hashchange", route);
  route();
})();

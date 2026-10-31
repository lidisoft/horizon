import { escapeHtml, html, raw, sheet } from "horizon/core/dom.js";
import { assert, describe, test } from "horizon-test/runner.js";

describe("dom", () => {
  test("escapeHtml escapes markup and quotes", () => {
    assert.equal(escapeHtml(`<a href="x">'&'</a>`), "&lt;a href=&quot;x&quot;&gt;&#39;&amp;&#39;&lt;/a&gt;");
    assert.equal(escapeHtml(null), "");
  });

  test("html escapes values but keeps raw() and nested templates", () => {
    const name = "<b>Ana</b>";
    const items = ["a", "<b>"].map((v) => html`<li>${v}</li>`);
    const out = String(html`<p title="${name}">${name} ${raw("<i>ok</i>")}</p><ul>${items}</ul>${false}${null}`);
    assert.equal(out, `<p title="&lt;b&gt;Ana&lt;/b&gt;">&lt;b&gt;Ana&lt;/b&gt; <i>ok</i></p><ul><li>a</li><li>&lt;b&gt;</li></ul>`);
  });

  test("sheet caches one CSSStyleSheet per text", () => {
    const a = sheet(":host { color: red; }");
    assert.ok(a instanceof CSSStyleSheet);
    assert.equal(sheet(":host { color: red; }"), a);
    assert.equal(a.cssRules.length, 1);
  });
});

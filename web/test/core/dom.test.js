import { escapeHtml, html, ldsLightSheet, raw, sheet } from "horizon/core/dom.js";
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
  test("ldsLightSheet brings LDS table and utility rules into shadow roots", async () => {
    const host = document.createElement("div");
    document.body.append(host);
    try {
      const root = host.attachShadow({ mode: "open" });
      root.adoptedStyleSheets = [ldsLightSheet()];
      root.innerHTML = `<lds-table><table><thead><tr><th>A</th><th><span class="lds-visually-hidden">B</span></th></tr></thead></table></lds-table>`;
      await customElements.whenDefined("lds-table");
      const th = root.querySelector("th");
      assert.equal(getComputedStyle(th).textAlign, "start");
      assert.ok(parseFloat(getComputedStyle(th).paddingLeft) > 0, "cells are padded");
      assert.equal(getComputedStyle(root.querySelector(".lds-visually-hidden")).position, "absolute");
      assert.equal(ldsLightSheet(), ldsLightSheet(), "one shared sheet");
      assert.ok(![...ldsLightSheet().cssRules].some((r) => r.selectorText === "body"), "no document rules");
    } finally {
      host.remove();
    }
  });
});

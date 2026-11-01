import { html } from "horizon/core/dom.js";
import { highlight } from "horizon/views/hzn-search.js";
import { assert, describe, test } from "horizon-test/runner.js";

describe("search", () => {
  test("highlight escapes the snippet and marks the matched words", () => {
    const out = String(html`${highlight("uses \u0002<SKIP>\u0003 and \u0002locked\u0003 rows")}`);
    assert.equal(out, "uses <mark>&lt;SKIP&gt;</mark> and <mark>locked</mark> rows");
    assert.equal(String(html`${highlight("plain")}`), "plain");
  });
});

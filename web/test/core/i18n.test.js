import { createI18n } from "horizon/core/i18n.js";
import { assert, describe, test } from "horizon-test/runner.js";

const catalog = {
  "hello": "Hello {name}",
  "issues.count": { one: "{count} issue", other: "{count} issues" },
  "zero.only": { other: "none" },
};

describe("i18n", () => {
  test("interpolates and leaves unknown placeholders", () => {
    const i18n = createI18n(catalog, { locale: "en" });
    assert.equal(i18n.t("hello", { name: "Ana" }), "Hello Ana");
    assert.equal(i18n.t("hello"), "Hello {name}");
  });

  test("picks the plural category and formats the count", () => {
    const i18n = createI18n(catalog, { locale: "en" });
    assert.equal(i18n.t("issues.count", { count: 1 }), "1 issue");
    assert.equal(i18n.t("issues.count", { count: 1200 }), "1,200 issues");
    assert.equal(i18n.t("zero.only", { count: 1 }), "none");
  });

  test("a missing key returns the key and is reported once", () => {
    const missing = [];
    const i18n = createI18n(catalog, { locale: "en", onMissing: (k) => missing.push(k) });
    assert.equal(i18n.t("nope"), "nope");
    i18n.t("nope");
    assert.deepEqual(missing, ["nope"]);
    assert.ok(i18n.has("hello") && !i18n.has("nope"));
  });

  test("formats dates in the given time zone and relative times", () => {
    const i18n = createI18n(catalog, { locale: "en-GB", timeZone: "Europe/Paris" });
    assert.equal(i18n.date("2026-10-27T23:30:00Z", { dateStyle: "short" }), "28/10/2026");
    const now = Date.parse("2026-10-30T12:00:00Z");
    assert.equal(i18n.relative("2026-10-27T12:00:00Z", now), "3 days ago");
    assert.equal(i18n.relative("2026-10-30T14:00:00Z", now), "in 2 hours");
    assert.equal(i18n.relative("2026-10-30T12:00:00Z", now), "now");
  });
});

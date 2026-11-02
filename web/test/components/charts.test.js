import { labelStep, niceTicks, refLines, shortLabel, stack } from "horizon/components/charts.js";
import { assert, describe, test } from "horizon-test/runner.js";

describe("charts", () => {
  test("niceTicks are 1, 2 or 5 × 10ⁿ apart, from 0 past the maximum", () => {
    assert.equal(niceTicks(10).join(), "0,5,10");
    assert.equal(niceTicks(9).join(), "0,5,10");
    assert.equal(niceTicks(37).join(), "0,10,20,30,40");
    assert.equal(niceTicks(0.7).join(), "0,0.2,0.4,0.6,0.8");
    assert.equal(niceTicks(0).join(), "0,1");
  });

  test("labelStep thins labels to fit", () => {
    assert.equal(labelStep(30, 600), 3);
    assert.equal(labelStep(5, 600), 1);
  });

  test("stack piles series, nulls left out", () => {
    const s = stack([{ values: [1, 2, null] }, { values: [3, 0, 4] }]);
    assert.equal(JSON.stringify(s), JSON.stringify([[[0, 1], [0, 2], null], [[1, 4], [2, 2], [0, 4]]]));
  });

  test("refLines merges lines at the same value", () => {
    const got = refLines([{ name: "p50", value: 2 }, { name: "p85", value: 2 }, { name: "p95", value: 5 }, { name: "x", value: null }]);
    assert.equal(got.map((r) => `${r.name}=${r.value}`).join(), "p50 · p85=2,p95=5");
  });

  test("shortLabel shortens dates only", () => {
    assert.equal(shortLabel("2026-11-02", "en"), "Nov 2");
    assert.equal(shortLabel("Sprint 3", "en"), "Sprint 3");
  });

  test("a line chart draws its series, a legend and a table", async () => {
    const el = document.createElement("hzn-chart-line");
    el.setAttribute("label", "Burndown");
    el.style.setProperty("width", "400px");
    document.getElementById("fixtures").append(el);
    el.data = { labels: ["2026-11-02", "2026-11-03", "2026-11-04"], series: [
      { name: "Remaining", tone: "started", values: [10, 8, null] },
      { name: "Ideal", tone: "reference", dashed: true, values: [10, 5, 0] },
    ] };
    await new Promise((r) => requestAnimationFrame(r));
    const root = el.shadowRoot;
    assert.equal(root.querySelectorAll("path.line").length, 2);
    el.data = { labels: ["a", "b", "c"], series: [{ name: "One", tone: "started", values: [null, 4, null] }] };
    assert.equal(root.querySelectorAll("circle.point").length, 1); // a lone value is a dot
    el.data = { labels: ["2026-11-02", "2026-11-03", "2026-11-04"], series: [
      { name: "Remaining", tone: "started", values: [10, 8, null] },
      { name: "Ideal", tone: "reference", dashed: true, values: [10, 5, 0] },
    ] };
    assert.equal(root.querySelector("path.line.t-started").getAttribute("d").split("L").length, 2); // the null day isn't drawn
    assert.equal(root.querySelectorAll(".legend span").length, 2);
    root.querySelector("[data-toggle]").click();
    assert.equal(root.querySelectorAll("tbody tr").length, 3);
    assert.equal(root.querySelector("tbody tr:last-child td").textContent, "–");
    el.remove();
  });
});

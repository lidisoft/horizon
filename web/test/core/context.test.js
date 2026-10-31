import { provideContext, requestContext } from "horizon/core/context.js";
import { assert, describe, test } from "horizon-test/runner.js";

describe("context", () => {
  test("a descendant, even inside a shadow root, gets the nearest provider's context", () => {
    const outer = document.createElement("div");
    const inner = document.createElement("div");
    provideContext(outer, { name: "outer" });
    provideContext(inner, { name: "inner" });
    const host = document.createElement("div");
    const leaf = document.createElement("span");
    host.attachShadow({ mode: "open" }).append(leaf);
    inner.append(host);
    outer.append(inner);
    document.getElementById("fixtures").append(outer);
    try {
      assert.equal(requestContext(leaf).name, "inner");
      assert.equal(requestContext(inner).name, "outer");
    } finally {
      outer.remove();
    }
  });

  test("without a provider it throws", () => {
    const lonely = document.createElement("hzn-lonely");
    document.getElementById("fixtures").append(lonely);
    try {
      assert.throws(() => requestContext(lonely), /no context provider/);
    } finally {
      lonely.remove();
    }
  });
});

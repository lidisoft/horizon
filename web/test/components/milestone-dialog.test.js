import { parseKeys } from "horizon/components/hzn-milestone-dialog.js";
import { assert, describe, test } from "horizon-test/runner.js";

describe("milestone dialog", () => {
  test("parseKeys reads issue keys however they are separated, once each", () => {
    assert.equal(parseKeys("hzn-1, HZN-2\nOPS-7;  hzn-1 nonsense -3").join(), "HZN-1,HZN-2,OPS-7");
    assert.equal(parseKeys("").length, 0);
  });
});

import { dateSpan, daysLeft, isOpen, localDate } from "horizon/components/planning-ui.js";
import { assert, describe, test } from "horizon-test/runner.js";

const i18n = { date: (d, o) => new Intl.DateTimeFormat("en-US", o).format(d) };

describe("planning-ui", () => {
  test("localDate reads a date without shifting it by the time zone", () => {
    const d = localDate("2026-11-09");
    assert.equal([d.getFullYear(), d.getMonth(), d.getDate()].join("-"), "2026-10-9");
  });

  test("dateSpan shows the iteration's dates, or nothing", () => {
    assert.equal(dateSpan({ start_date: "2026-11-09", end_date: "2026-11-20" }, i18n), "Nov 9 – Nov 20");
    assert.equal(dateSpan({ start_date: null, end_date: null }, i18n), "");
  });

  test("daysLeft counts whole days to the end date", () => {
    const now = new Date(2026, 10, 9, 23, 30);
    assert.equal(daysLeft({ end_date: "2026-11-20" }, now), 11);
    assert.equal(daysLeft({ end_date: "2026-11-09" }, now), 0);
    assert.equal(daysLeft({ end_date: "2026-11-08" }, now), -1);
    assert.equal(daysLeft({ end_date: null }, now), null);
  });

  test("isOpen is true while issues can be loaded", () => {
    assert.equal(["planned", "scheduled", "active", "completed", "archived"].map((state) => isOpen({ state })).join(), "true,true,true,false,false");
  });
});

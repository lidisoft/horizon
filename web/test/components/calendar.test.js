import { Calendar, addDays, daysBetween, isoWeekday, today } from "horizon/components/calendar.js";
import { assert, describe, test } from "horizon-test/runner.js";

describe("working calendar", () => {
  // 2026-12-21 is a Monday; Christmas is a Friday. The same cases as internal/calendar.
  const c = new Calendar("Europe/Paris", [1, 2, 3, 4, 5], [{ date: "2026-12-25", name: "Christmas" }]);

  test("dates are plain days", () => {
    assert.equal(addDays("2026-12-31", 1), "2027-01-01");
    assert.equal(addDays("2026-03-29", -1), "2026-03-28"); // across a DST change
    assert.equal(isoWeekday("2026-12-27"), 7);
    assert.equal(daysBetween("2026-12-21", "2027-01-03"), 13);
  });

  test("working days skip weekends and holidays", () => {
    assert.equal(c.working("2026-12-24"), true);
    assert.equal(c.working("2026-12-25"), false);
    assert.equal(c.working("2026-12-26"), false);
    assert.equal(c.holiday("2026-12-25"), "Christmas");
    assert.equal(c.add("2026-12-24", 1), "2026-12-28");
    assert.equal(c.add("2026-12-28", -1), "2026-12-24");
    assert.equal(c.add("2026-12-26", 1), "2026-12-28");
    assert.equal(c.add("2026-12-21", 5), "2026-12-29");
    assert.equal(c.between("2026-12-21", "2027-01-03"), 9);
    assert.equal(c.next("2026-12-25", -1), "2026-12-24");
  });

  test("today is in the calendar's time zone", () => {
    const now = new Date("2026-12-31T23:30:00Z");
    assert.equal(c.today(now), "2027-01-01");
    assert.equal(today("UTC", now), "2026-12-31");
  });

  test("from resolved settings, with defaults", () => {
    const gulf = Calendar.from({ time_zone: { value: "Asia/Dubai" }, working_days: { value: [7, 1, 2, 3, 4] }, holidays: { value: [] } });
    assert.equal(gulf.working("2026-12-27"), true);
    assert.equal(gulf.working("2026-12-25"), false);
    assert.equal(Calendar.from(null).working("2026-12-26"), false);
  });
});

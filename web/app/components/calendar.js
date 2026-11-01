// A project's working calendar (docs/configuration.md#time-zone-and-working-calendar),
// mirroring internal/calendar: working weekdays (ISO, Monday 1 … Sunday 7),
// non-working dates, and arithmetic in working days. Dates are "YYYY-MM-DD"
// strings, computed in UTC so no local time zone shifts them.

const MAX_SCAN = 3660; // ten years of days: a calendar with no working day can't loop

/** "YYYY-MM-DD" ± days. */
export function addDays(date, days) {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

/** 1 (Monday) to 7 (Sunday). */
export function isoWeekday(date) {
  return new Date(`${date}T00:00:00Z`).getUTCDay() || 7;
}

/** Days from a to b ("YYYY-MM-DD"); negative when b is before a. */
export function daysBetween(a, b) {
  return Math.round((Date.parse(`${b}T00:00:00Z`) - Date.parse(`${a}T00:00:00Z`)) / 86400000);
}

/** Today in a time zone, as "YYYY-MM-DD". */
export function today(timeZone, now = new Date()) {
  try {
    return new Intl.DateTimeFormat("en-CA", { timeZone, year: "numeric", month: "2-digit", day: "2-digit" }).format(now);
  } catch {
    return now.toISOString().slice(0, 10);
  }
}

export class Calendar {
  /** From the resolved configuration's calendar: { time_zone, working_days, holidays }. */
  static from(settings) {
    return new Calendar(settings?.time_zone?.value ?? "UTC", settings?.working_days?.value, settings?.holidays?.value);
  }

  constructor(timeZone = "UTC", weekdays = [1, 2, 3, 4, 5], holidays = []) {
    this.timeZone = timeZone;
    this.weekdays = new Set(weekdays?.length ? weekdays : [1, 2, 3, 4, 5]);
    this.holidays = new Map((holidays ?? []).map((h) => [h.date, h.name]));
  }

  today(now = new Date()) {
    return today(this.timeZone, now);
  }

  /** The name of the non-working date, or undefined. */
  holiday(date) {
    return this.holidays.get(date);
  }

  working(date) {
    return this.weekdays.has(isoWeekday(date)) && !this.holidays.has(date);
  }

  /** date if worked, else the first working day after (dir 1) or before (dir -1). */
  next(date, dir = 1) {
    for (let i = 0; i < MAX_SCAN && !this.working(date); i++) date = addDays(date, dir);
    return date;
  }

  /** n working days from date (back when n < 0). */
  add(date, n) {
    const dir = n < 0 ? -1 : 1;
    let left = Math.abs(n);
    for (let i = 0; left > 0 && i < MAX_SCAN; i++) {
      date = addDays(date, dir);
      if (this.working(date)) left--;
    }
    return date;
  }

  /** Working days from from to to, both included (0 when to is before from). */
  between(from, to) {
    let n = 0;
    for (let d = from, i = 0; d <= to && i < MAX_SCAN; d = addDays(d, 1), i++) if (this.working(d)) n++;
    return n;
  }
}

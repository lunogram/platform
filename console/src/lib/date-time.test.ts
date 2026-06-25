// Unit tests for the pure local-datetime helpers. These helpers treat the
// date+time inputs as *local* wall-clock time (the browser's timezone) and only
// convert to UTC at the ISO boundary, so the tests assert round-trip and
// structural properties that hold regardless of the machine's timezone.

import { describe, expect, it } from "vitest"

import {
    DEFAULT_TIME_INPUT_VALUE,
    dateInputValueFromDate,
    dateInputValueFromIso,
    parseDateAndTime,
    splitLocalDateTimeValue,
    timeInputValueFromIso,
    toIsoFromDateAndTime,
    toLocalDateTimeValue,
} from "./date-time"

describe("parseDateAndTime", () => {
    it("returns null for an empty date", () => {
        expect(parseDateAndTime("")).toBeNull()
        expect(parseDateAndTime("", "10:30")).toBeNull()
    })

    it("parses a date + time as local wall-clock time", () => {
        const parsed = parseDateAndTime("2026-06-24", "10:30")
        expect(parsed).not.toBeNull()
        expect(parsed!.getFullYear()).toBe(2026)
        expect(parsed!.getMonth()).toBe(5) // June, zero-indexed
        expect(parsed!.getDate()).toBe(24)
        expect(parsed!.getHours()).toBe(10)
        expect(parsed!.getMinutes()).toBe(30)
    })

    it("defaults the time to midnight when omitted", () => {
        const parsed = parseDateAndTime("2026-06-24")
        expect(parsed!.getHours()).toBe(0)
        expect(parsed!.getMinutes()).toBe(0)
    })

    it("treats an empty time string as midnight", () => {
        const parsed = parseDateAndTime("2026-06-24", "")
        expect(parsed!.getHours()).toBe(0)
        expect(parsed!.getMinutes()).toBe(0)
    })

    it("returns null for an invalid date", () => {
        expect(parseDateAndTime("not-a-date", "10:30")).toBeNull()
    })
})

describe("toIsoFromDateAndTime", () => {
    it("produces an ISO string for a valid date", () => {
        const iso = toIsoFromDateAndTime("2026-06-24", "10:30")
        expect(iso).toBeDefined()
        // ISO is always UTC ("Z"), independent of the local timezone.
        expect(iso).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/)
    })

    it("round-trips back to the same local wall-clock time", () => {
        const iso = toIsoFromDateAndTime("2026-06-24", "10:30")!
        const back = new Date(iso)
        expect(dateInputValueFromIso(iso)).toBe("2026-06-24")
        expect(timeInputValueFromIso(iso)).toBe("10:30")
        expect(back.getHours()).toBe(10)
        expect(back.getMinutes()).toBe(30)
    })

    it("returns undefined for an empty date", () => {
        expect(toIsoFromDateAndTime("")).toBeUndefined()
    })
})

describe("dateInputValueFromDate", () => {
    it("formats a date as YYYY-MM-DD with zero padding", () => {
        const date = new Date(2026, 0, 5) // 5 Jan 2026 local
        expect(dateInputValueFromDate(date)).toBe("2026-01-05")
    })

    it("returns an empty string for an invalid date", () => {
        expect(dateInputValueFromDate(new Date("nope"))).toBe("")
    })
})

describe("dateInputValueFromIso / timeInputValueFromIso", () => {
    it("returns empty / default for an invalid ISO string", () => {
        expect(dateInputValueFromIso("garbage")).toBe("")
        expect(timeInputValueFromIso("garbage")).toBe(DEFAULT_TIME_INPUT_VALUE)
    })
})

describe("toLocalDateTimeValue", () => {
    it("combines date and time into a datetime-local value", () => {
        expect(toLocalDateTimeValue("2026-06-24", "10:30")).toBe("2026-06-24T10:30")
    })

    it("defaults the time to midnight when omitted or empty", () => {
        expect(toLocalDateTimeValue("2026-06-24")).toBe("2026-06-24T00:00")
        expect(toLocalDateTimeValue("2026-06-24", "")).toBe("2026-06-24T00:00")
    })

    it("returns an empty string when the date is empty", () => {
        expect(toLocalDateTimeValue("")).toBe("")
        expect(toLocalDateTimeValue("", "10:30")).toBe("")
    })
})

describe("splitLocalDateTimeValue", () => {
    it("splits a datetime-local value into its parts", () => {
        expect(splitLocalDateTimeValue("2026-06-24T10:30")).toEqual({
            dateValue: "2026-06-24",
            timeValue: "10:30",
        })
    })

    it("ignores seconds in the time portion", () => {
        expect(splitLocalDateTimeValue("2026-06-24T10:30:45")).toEqual({
            dateValue: "2026-06-24",
            timeValue: "10:30",
        })
    })

    it("returns empty date and default time for an empty value", () => {
        expect(splitLocalDateTimeValue("")).toEqual({
            dateValue: "",
            timeValue: DEFAULT_TIME_INPUT_VALUE,
        })
    })

    it("round-trips with toLocalDateTimeValue", () => {
        const combined = toLocalDateTimeValue("2026-06-24", "10:30")
        expect(splitLocalDateTimeValue(combined)).toEqual({
            dateValue: "2026-06-24",
            timeValue: "10:30",
        })
    })
})

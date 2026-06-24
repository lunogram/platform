export const DEFAULT_TIME_INPUT_VALUE = "00:00"

function isValidDate(date: Date): boolean {
    return !Number.isNaN(date.getTime())
}

function normalizeTimeInputValue(value: string): string {
    return value || DEFAULT_TIME_INPUT_VALUE
}

function toDateInputValue(date: Date): string {
    const year = date.getFullYear()
    const month = String(date.getMonth() + 1).padStart(2, "0")
    const day = String(date.getDate()).padStart(2, "0")
    return `${year}-${month}-${day}`
}

function toTimeInputValue(date: Date): string {
    const hours = String(date.getHours()).padStart(2, "0")
    const minutes = String(date.getMinutes()).padStart(2, "0")
    return `${hours}:${minutes}`
}

export function parseDateAndTime(
    dateValue: string,
    timeValue: string = DEFAULT_TIME_INPUT_VALUE,
): Date | null {
    if (!dateValue) return null

    const parsed = new Date(`${dateValue}T${normalizeTimeInputValue(timeValue)}`)
    return isValidDate(parsed) ? parsed : null
}

export function toIsoFromDateAndTime(
    dateValue: string,
    timeValue: string = DEFAULT_TIME_INPUT_VALUE,
): string | undefined {
    const parsed = parseDateAndTime(dateValue, timeValue)
    return parsed?.toISOString()
}

export function dateInputValueFromDate(date: Date): string {
    return isValidDate(date) ? toDateInputValue(date) : ""
}

export function dateInputValueFromIso(iso: string): string {
    const date = new Date(iso)
    return isValidDate(date) ? toDateInputValue(date) : ""
}

export function timeInputValueFromIso(iso: string): string {
    const date = new Date(iso)
    return isValidDate(date) ? toTimeInputValue(date) : DEFAULT_TIME_INPUT_VALUE
}

export function toLocalDateTimeValue(
    dateValue: string,
    timeValue: string = DEFAULT_TIME_INPUT_VALUE,
): string {
    if (!dateValue) return ""
    return `${dateValue}T${normalizeTimeInputValue(timeValue)}`
}

export function splitLocalDateTimeValue(value: string): {
    dateValue: string
    timeValue: string
} {
    return {
        dateValue: value ? value.slice(0, 10) : "",
        timeValue: value ? value.slice(11, 16) : DEFAULT_TIME_INPUT_VALUE,
    }
}

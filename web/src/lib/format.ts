import type { Reason, RotationUnit, Service, Unavailability } from "@/lib/api"
import { LOCALE, t } from "@/lib/i18n"

const WEEKDAY_KEYS = [
  "weekday.sunday",
  "weekday.monday",
  "weekday.tuesday",
  "weekday.wednesday",
  "weekday.thursday",
  "weekday.friday",
  "weekday.saturday",
] as const

const WEEKDAY_SHORT_KEYS = [
  "weekdayShort.sunday",
  "weekdayShort.monday",
  "weekdayShort.tuesday",
  "weekdayShort.wednesday",
  "weekdayShort.thursday",
  "weekdayShort.friday",
  "weekdayShort.saturday",
] as const

export const WEEKDAY_INDEXES = [0, 1, 2, 3, 4, 5, 6] as const

export const REASONS: readonly Reason[] = [
  "viagem",
  "trabalho",
  "saude",
  "familia",
  "estudos",
  "outro",
]

export function weekdayName(index: number): string {
  return t(WEEKDAY_KEYS[index])
}

export function rotationLabel(unit: RotationUnit): string {
  return t(`format.rotation.${unit}`)
}

export function reasonLabel(reason: Reason): string {
  return t(`format.reason.${reason}`)
}

export function formatPhone(digits: string): string {
  const match = /^55(\d{2})(\d{4,5})(\d{4})$/.exec(digits)
  if (match) {
    return `(${match[1]}) ${match[2]}-${match[3]}`
  }
  return digits ? `+${digits}` : ""
}

/**
 * NOTE: "19:00" reads as "19h" and "19:30" as "19h30", the way the team writes it on WhatsApp.
 */
export function formatTime(startTime: string): string {
  const [hours, minutes] = startTime.split(":")
  return minutes === "00"
    ? t("format.timeOnTheHour", { hours })
    : t("format.time", { hours, minutes })
}

export function serviceLabel(
  service: Pick<Service, "weekday" | "startTime">
): string {
  return t("format.serviceWhen", {
    weekday: weekdayName(service.weekday),
    time: formatTime(service.startTime),
  })
}

export function firstName(name: string): string {
  return name.split(" ")[0] ?? name
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/)
  const first = parts[0]?.charAt(0) ?? ""
  const last = parts.length > 1 ? parts[parts.length - 1].charAt(0) : ""
  return (first + last).toUpperCase()
}

function parseDate(date: string): Date {
  const [year, month, day] = date.split("-").map(Number)
  return new Date(year, month - 1, day)
}

export function formatDay(date: string): string {
  const [, month, day] = date.split("-")
  return `${day}/${month}`
}

export function dayOfMonth(date: string): string {
  return date.split("-")[2]
}

export function weekdayShort(date: string): string {
  return weekdayShortName(parseDate(date).getDay())
}

export function weekdayShortName(index: number): string {
  return t(WEEKDAY_SHORT_KEYS[index])
}

export function longDate(date: string): string {
  return new Intl.DateTimeFormat(LOCALE, { weekday: "long", day: "numeric", month: "long" }).format(parseDate(date))
}

export function thisMonth(): string {
  return todayISO().slice(0, 7)
}

export function weekdayOf(date: string): string {
  return weekdayName(parseDate(date).getDay())
}

export function dayAndTime(date: string, startTime: string): string {
  return t("format.dayAndTime", {
    weekday: weekdayOf(date),
    day: formatDay(date),
    time: formatTime(startTime),
  })
}

export function weekRange(start: string): string {
  const end = parseDate(start)
  end.setDate(end.getDate() + 6)
  const pad = (n: number) => String(n).padStart(2, "0")
  return t("format.dayRange", {
    from: formatDay(start),
    to: `${pad(end.getDate())}/${pad(end.getMonth() + 1)}`,
  })
}

export function monthLabel(month: string): string {
  const label = new Intl.DateTimeFormat(LOCALE, {
    month: "long",
    year: "numeric",
  }).format(parseDate(`${month}-01`))
  return label.charAt(0).toUpperCase() + label.slice(1)
}

export function shiftMonth(month: string, delta: number): string {
  const date = parseDate(`${month}-01`)
  date.setMonth(date.getMonth() + delta)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`
}

export function nextMonth(): string {
  const today = new Date()
  return shiftMonth(
    `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, "0")}`,
    1
  )
}

export function todayISO(): string {
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
}

export function monthBounds(month: string): { from: string; to: string } {
  const last = parseDate(`${shiftMonth(month, 1)}-01`)
  last.setDate(last.getDate() - 1)
  return {
    from: `${month}-01`,
    to: `${month}-${String(last.getDate()).padStart(2, "0")}`,
  }
}

export function formatDateRange(start: string, end: string): string {
  return start === end
    ? formatDay(start)
    : t("format.dayRange", { from: formatDay(start), to: formatDay(end) })
}

function masculine(weekday: number): boolean {
  return weekday === 0 || weekday === 6
}

/**
 * NOTE: How an absence reads in a list: "03/12 a 31/12", "Toda quinta até 31/03" or "Todo 1º domingo do mês".
 */
export function absenceLabel(
  u: Pick<
    Unavailability,
    "startDate" | "endDate" | "repeat" | "weekday" | "weekOfMonth"
  >
): string {
  if (u.repeat === "none" || u.weekday === null) {
    return formatDateRange(u.startDate, u.endDate)
  }
  const weekday = weekdayName(u.weekday).toLowerCase()
  const isMasculine = masculine(u.weekday)
  let label = t(
    isMasculine
      ? "format.absence.everyMasculine"
      : "format.absence.everyFeminine",
    { weekday }
  )
  if (u.repeat === "monthly" && u.weekOfMonth) {
    const week = u.weekOfMonth
    label = t(
      isMasculine
        ? "format.absence.monthlyMasculine"
        : "format.absence.monthlyFeminine",
      { week, weekday }
    )
  }
  return u.endDate
    ? t("format.absence.until", { label, day: formatDay(u.endDate) })
    : label
}

/**
 * NOTE: Mirrors the server: a repeating absence only covers its weekday (and week of the month) inside its range.
 */
export function absenceCovers(u: Unavailability, date: string): boolean {
  if (date < u.startDate || (u.endDate && date > u.endDate)) {
    return false
  }
  if (u.repeat === "none") {
    return true
  }
  const day = parseDate(date)
  if (day.getDay() !== u.weekday) {
    return false
  }
  return (
    u.repeat === "weekly" ||
    Math.floor((day.getDate() - 1) / 7) + 1 === u.weekOfMonth
  )
}

export function communionLabel(
  service: Pick<Service, "weekday" | "communionWeek">
): string | null {
  if (!service.communionWeek) {
    return null
  }
  const key = masculine(service.weekday) ? "format.communionMasculine" : "format.communionFeminine"
  return t(key, {
    week: service.communionWeek,
    weekday: weekdayName(service.weekday).toLowerCase(),
  })
}

export function ordinalOfMonth(week: number, weekday: number): string {
  const key = masculine(weekday) ? "format.ordinalMasculine" : "format.ordinalFeminine"
  return t(key, {
    week,
    weekday: weekdayName(weekday).toLowerCase(),
  })
}

/**
 * NOTE: Formats a Brazilian number while it is typed: "(75) 99999-9999", or "(75) 9999-9999" for landlines.
 */
export function maskPhone(input: string): string {
  const digits = input.replace(/\D/g, "").slice(0, 11)
  if (digits.length <= 2) {
    return digits.length ? `(${digits}` : ""
  }
  const area = digits.slice(0, 2)
  const rest = digits.slice(2)
  const split = rest.length > 8 ? 5 : 4
  return rest.length > split
    ? `(${area}) ${rest.slice(0, split)}-${rest.slice(split)}`
    : `(${area}) ${rest}`
}

export function phoneDigits(input: string): string {
  return input.replace(/\D/g, "")
}

const names = new Intl.ListFormat(LOCALE, { style: "long", type: "conjunction" })

/**
 * NOTE: "Ana", "Ana e Bia", "Ana, Bia e Caio".
 */
export function listNames(list: string[]): string {
  return names.format(list)
}

import type { RotationUnit, Service } from "@/lib/api"

export const WEEKDAYS = ["Domingo", "Segunda", "Terça", "Quarta", "Quinta", "Sexta", "Sábado"] as const

export const ROTATION_LABEL: Record<RotationUnit, string> = {
  week: "Rodízio por semana",
  service: "Rodízio por culto",
}

export function formatPhone(digits: string): string {
  const match = /^55(\d{2})(\d{4,5})(\d{4})$/.exec(digits)
  if (match) {
    return `(${match[1]}) ${match[2]}-${match[3]}`
  }
  return digits ? `+${digits}` : ""
}

export function serviceLabel(service: Pick<Service, "weekday" | "startTime">): string {
  return `${WEEKDAYS[service.weekday]}, ${service.startTime.replace(":", "h")}`
}

export function firstName(name: string): string {
  return name.split(" ")[0] ?? name
}

function parseDate(date: string): Date {
  const [year, month, day] = date.split("-").map(Number)
  return new Date(year, month - 1, day)
}

export function formatDay(date: string): string {
  const [, month, day] = date.split("-")
  return `${day}/${month}`
}

export function weekdayOf(date: string): string {
  return WEEKDAYS[parseDate(date).getDay()]
}

export function weekRange(start: string): string {
  const end = parseDate(start)
  end.setDate(end.getDate() + 6)
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${formatDay(start)} a ${pad(end.getDate())}/${pad(end.getMonth() + 1)}`
}

export function monthLabel(month: string): string {
  const label = new Intl.DateTimeFormat("pt-BR", { month: "long", year: "numeric" }).format(parseDate(`${month}-01`))
  return label.charAt(0).toUpperCase() + label.slice(1)
}

export function shiftMonth(month: string, delta: number): string {
  const date = parseDate(`${month}-01`)
  date.setMonth(date.getMonth() + delta)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`
}

export function nextMonth(): string {
  const today = new Date()
  return shiftMonth(`${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, "0")}`, 1)
}

export function monthBounds(month: string): { from: string; to: string } {
  const last = parseDate(`${shiftMonth(month, 1)}-01`)
  last.setDate(last.getDate() - 1)
  return { from: `${month}-01`, to: `${month}-${String(last.getDate()).padStart(2, "0")}` }
}

export function formatDateRange(start: string, end: string): string {
  return start === end ? formatDay(start) : `${formatDay(start)} a ${formatDay(end)}`
}

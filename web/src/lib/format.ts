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

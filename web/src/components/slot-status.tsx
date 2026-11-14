import { Check, Hourglass, X } from "lucide-react"
import { useTranslation } from "react-i18next"

import type { AssignmentStatus } from "@/lib/api"
import { cn } from "@/lib/utils"

/**
 * NOTE: A person's answer as an icon plus hidden text, so the meaning never rests on colour or strike-through alone.
 */
export function SlotStatusIcon({ status, coverOpen, className }: { status: AssignmentStatus; coverOpen: boolean; className?: string }) {
  const { t } = useTranslation()
  if (coverOpen) {
    return (
      <span className={cn("inline-flex text-warning", className)}>
        <Hourglass className="size-3.5" aria-hidden />
        <span className="sr-only">{t("status.searching")}</span>
      </span>
    )
  }
  if (status === "confirmed") {
    return (
      <span className={cn("inline-flex text-success", className)}>
        <Check className="size-3.5" strokeWidth={2.75} aria-hidden />
        <span className="sr-only">{t("status.confirmed")}</span>
      </span>
    )
  }
  if (status === "declined") {
    return (
      <span className={cn("inline-flex text-danger", className)}>
        <X className="size-3.5" strokeWidth={2.75} aria-hidden />
        <span className="sr-only">{t("status.declined")}</span>
      </span>
    )
  }
  return (
    <span className={cn("inline-flex size-3.5 items-center justify-center", className)}>
      <span className="size-1.5 rounded-full bg-muted-foreground/60" aria-hidden />
      <span className="sr-only">{t("status.pending")}</span>
    </span>
  )
}

import { Grape } from "lucide-react"
import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"

export function CommunionTag({ className }: { className?: string }) {
  const { t } = useTranslation()
  return (
    <span className={cn("inline-flex items-center gap-1 text-xs font-medium text-brand", className)}>
      <Grape className="size-3.5" aria-hidden />
      {t("format.communionShort")}
    </span>
  )
}

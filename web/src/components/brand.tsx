import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"

export function Wordmark({ className }: { className?: string }) {
  const { t } = useTranslation()
  return (
    <span className={cn("flex items-center gap-2.5", className)}>
      <img src="/favicon.svg" alt="" className="size-8 rounded-[9px] shadow-[0_2px_8px_oklch(0.56_0.2_260/0.35)]" />
      <span className="font-display text-[1.375rem] leading-none font-semibold">{t("app.name")}</span>
    </span>
  )
}

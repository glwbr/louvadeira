import { useTranslation } from "react-i18next"

import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"

/**
 * NOTE: Placeholders shaped like the rows they stand for (leading mark, title, detail), so the page does not jump
 * when the data arrives.
 */
export function ListSkeleton({ rows = 3, className }: { rows?: number; className?: string }) {
  const { t } = useTranslation()
  return (
    <div role="status" aria-label={t("common.loading")} className={cn("divide-y rounded-2xl border bg-card", className)}>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="flex items-center gap-3 px-4 py-3.5">
          <Skeleton className="size-10 shrink-0 rounded-xl" />
          <div className="flex flex-1 flex-col gap-2">
            <Skeleton className="h-4" style={{ width: `${62 - ((i * 17) % 30)}%` }} />
            <Skeleton className="h-3" style={{ width: `${38 - ((i * 11) % 16)}%` }} />
          </div>
        </div>
      ))}
    </div>
  )
}

export function CardSkeleton({ lines = 3, className }: { lines?: number; className?: string }) {
  const { t } = useTranslation()
  return (
    <div role="status" aria-label={t("common.loading")} className={cn("flex gap-4 rounded-2xl border bg-card p-4", className)}>
      <Skeleton className="h-16 w-14 shrink-0 rounded-xl" />
      <div className="flex flex-1 flex-col gap-2.5 pt-1">
        <Skeleton className="h-4 w-1/2" />
        {Array.from({ length: lines - 1 }, (_, i) => (
          <Skeleton key={i} className="h-3" style={{ width: `${70 - i * 18}%` }} />
        ))}
      </div>
    </div>
  )
}

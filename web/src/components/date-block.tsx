import { dayOfMonth, formatTime, weekdayShort } from "@/lib/format"
import { cn } from "@/lib/utils"

/**
 * NOTE: The date set the way a printed escala shows it: the day of the month large, weekday and time beside it. The
 * size stays fixed so stacked services line up.
 */
export function DateBlock({ date, startTime, muted = false, className }: { date: string; startTime?: string; muted?: boolean; className?: string }) {
  return (
    <div
      className={cn(
        "flex w-14 shrink-0 flex-col items-center self-start rounded-xl border bg-background/40 py-1.5 leading-none",
        muted && "opacity-60",
        className
      )}
    >
      <span className="text-[0.6875rem] font-semibold tracking-wide text-brand uppercase">{weekdayShort(date)}</span>
      <span className="tabular mt-1 font-display text-[1.625rem] font-semibold">{dayOfMonth(date)}</span>
      {startTime && <span className="tabular mt-1 text-[0.6875rem] text-muted-foreground">{formatTime(startTime)}</span>}
    </div>
  )
}

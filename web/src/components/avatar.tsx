import { initials } from "@/lib/format"
import { cn } from "@/lib/utils"

export function Avatar({ name, active = false, className }: { name: string; active?: boolean; className?: string }) {
  return (
    <span
      aria-hidden
      className={cn(
        "flex size-10 shrink-0 items-center justify-center rounded-full text-sm font-semibold",
        active ? "bg-primary text-primary-foreground" : "bg-secondary text-secondary-foreground",
        className
      )}
    >
      {initials(name)}
    </span>
  )
}

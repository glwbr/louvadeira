import { ChevronRight } from "lucide-react"
import type { ReactNode } from "react"
import { Link } from "react-router"

import { AnimatedList } from "@/components/motion"
import { cn } from "@/lib/utils"

/**
 * NOTE: The grouped card most lists sit in. Its rows are AnimatedItems, so additions and removals animate.
 */
export function ListCard({ children, className, label }: { children: ReactNode; className?: string; label?: string }) {
  return (
    <ul aria-label={label} className={cn("flex flex-col divide-y overflow-hidden rounded-2xl border bg-card", className)}>
      <AnimatedList>{children}</AnimatedList>
    </ul>
  )
}

type RowProps = {
  leading?: ReactNode
  title: ReactNode
  subtitle?: ReactNode
  trailing?: ReactNode
  chevron?: boolean
  className?: string
} & ({ to: string; onClick?: never } | { onClick: () => void; to?: never })

const ROW = "group flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-accent/40 focus-visible:bg-accent/40 focus-visible:outline-none"

/**
 * NOTE: One tappable row: leading mark, title over subtitle, trailing badges and a chevron that nudges on hover.
 */
export function ListRow({ leading, title, subtitle, trailing, chevron = true, className, ...action }: RowProps) {
  const body = (
    <>
      {leading}
      <span className="min-w-0 flex-1">
        <span className="block font-medium text-pretty">{title}</span>
        {subtitle && <span className="block text-sm text-muted-foreground">{subtitle}</span>}
      </span>
      {trailing}
      {chevron && <ChevronRight className="size-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5" aria-hidden />}
    </>
  )
  if (action.to !== undefined) {
    return (
      <Link to={action.to} className={cn(ROW, className)}>
        {body}
      </Link>
    )
  }
  return (
    <button type="button" onClick={action.onClick} className={cn(ROW, className)}>
      {body}
    </button>
  )
}

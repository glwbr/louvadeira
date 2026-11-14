import type { LucideIcon } from "lucide-react"
import type { ReactNode } from "react"

import { Reveal } from "@/components/motion"
import { cn } from "@/lib/utils"

type Props = {
  icon: LucideIcon
  title: ReactNode
  children?: ReactNode
  action?: ReactNode
  tone?: "brand" | "danger"
  className?: string
}

export function EmptyState({ icon: Icon, title, children, action, tone = "brand", className }: Props) {
  return (
    <Reveal
      className={cn(
        "flex flex-col items-center gap-3 rounded-2xl border border-dashed px-6 py-10 text-center",
        tone === "danger" && "border-danger/30",
        className
      )}
    >
      <span
        className={cn(
          "flex size-12 items-center justify-center rounded-2xl",
          tone === "brand" ? "bg-primary/12 text-brand" : "bg-danger/12 text-danger"
        )}
      >
        <Icon className="size-6" />
      </span>
      <p className="font-medium text-balance">{title}</p>
      {children && <div className="max-w-sm text-sm text-pretty text-muted-foreground">{children}</div>}
      {action && <div className="mt-2">{action}</div>}
    </Reveal>
  )
}

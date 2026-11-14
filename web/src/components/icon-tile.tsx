import type { LucideIcon } from "lucide-react"

import { cn } from "@/lib/utils"

export function IconTile({ icon: Icon, className }: { icon: LucideIcon; className?: string }) {
  return (
    <span aria-hidden className={cn("flex size-11 shrink-0 items-center justify-center rounded-xl bg-primary/12 text-brand", className)}>
      <Icon className="size-5" />
    </span>
  )
}

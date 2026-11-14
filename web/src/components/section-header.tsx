import type { LucideIcon } from "lucide-react"
import type { ReactNode } from "react"

export function SectionHeader({ id, title, icon: Icon, action }: { id: string; title: ReactNode; icon?: LucideIcon; action?: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <h2 id={id} className="flex items-center gap-2 text-lg font-semibold">
        {Icon && <Icon className="size-5 text-brand" aria-hidden />}
        {title}
      </h2>
      {action}
    </div>
  )
}

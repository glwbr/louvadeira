import type { ReactNode } from "react"

export function PageHeader({ title, description, action }: { title: ReactNode; description?: ReactNode; action?: ReactNode }) {
  return (
    <div className="mb-6 flex flex-wrap items-end justify-between gap-x-4 gap-y-3">
      <div className="min-w-0 flex-1 basis-56">
        <h1 className="font-display text-[2rem] leading-[1.1] font-semibold text-balance">{title}</h1>
        {description && <p className="mt-1.5 max-w-prose text-sm text-pretty text-muted-foreground">{description}</p>}
      </div>
      {action && <div className="flex shrink-0 items-center gap-2">{action}</div>}
    </div>
  )
}

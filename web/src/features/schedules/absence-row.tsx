import { ArrowLeftRight, X } from "lucide-react"
import type { ReactNode } from "react"

import { ConfirmButton } from "@/components/confirm-button"
import { AnimatedItem } from "@/components/motion"
import { Button } from "@/components/ui/button"

type Props = {
  leading: ReactNode
  title: ReactNode
  subtitle: ReactNode
  /** NOTE: Offered while the absence still falls on services in a published schedule. */
  cover?: { label: string; onClick: () => void }
  remove: { label: string; title: string; description: string; confirmLabel: string; onConfirm: () => void; pending: boolean }
}

/**
 * NOTE: One absence in a list, the same for the leader and for the member: who or when, why, a way to find a
 * substitute and a way to take it back.
 */
export function AbsenceRow({ leading, title, subtitle, cover, remove }: Props) {
  return (
    <AnimatedItem className="flex flex-col gap-2 px-4 py-3">
      <div className="flex items-center gap-3">
        {leading}
        <div className="min-w-0 flex-1">
          <p className="truncate font-medium">{title}</p>
          <p className="text-xs text-pretty text-muted-foreground">{subtitle}</p>
        </div>
        <ConfirmButton
          size="icon-sm"
          icon={<X />}
          label={null}
          aria-label={remove.label}
          confirmLabel={remove.confirmLabel}
          title={remove.title}
          description={remove.description}
          onConfirm={remove.onConfirm}
          pending={remove.pending}
          className="text-muted-foreground hover:text-danger"
        />
      </div>
      {cover && (
        <Button size="sm" variant="outline" className="self-start sm:ml-12" onClick={cover.onClick}>
          <ArrowLeftRight />
          {cover.label}
        </Button>
      )}
    </AnimatedItem>
  )
}

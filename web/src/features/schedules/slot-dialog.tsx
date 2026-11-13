import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Check } from "lucide-react"
import { toast } from "sonner"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import {
  api,
  queryKeys,
  type MinistryDetail,
  type ScheduleRole,
  type ScheduleService,
  type ScheduleSlot,
  type Unavailability,
} from "@/lib/api"
import { toastError } from "@/lib/errors"
import { formatDay, weekdayOf } from "@/lib/format"
import { cn } from "@/lib/utils"

export type SlotSelection = {
  service: ScheduleService
  role: ScheduleRole
  slot: ScheduleSlot
}

type Props = {
  selection?: SlotSelection
  onClose: () => void
  ministry: MinistryDetail
  month: string
  unavailabilities: Unavailability[]
}

export function SlotDialog({ selection, onClose, ministry, month, unavailabilities }: Props) {
  return (
    <Dialog open={Boolean(selection)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        {selection && (
          <SlotPicker
            selection={selection}
            ministry={ministry}
            month={month}
            unavailabilities={unavailabilities}
            onDone={onClose}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

function SlotPicker({
  selection,
  ministry,
  month,
  unavailabilities,
  onDone,
}: {
  selection: SlotSelection
  ministry: MinistryDetail
  month: string
  unavailabilities: Unavailability[]
  onDone: () => void
}) {
  const queryClient = useQueryClient()
  const { service, role, slot } = selection
  const assign = useMutation({
    mutationFn: (personId: number | null) => api.schedules.assign(ministry.id, month, slot.assignmentId, personId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.schedule(ministry.id, month) })
      toast.success("Escala atualizada")
      onDone()
    },
    onError: toastError,
  })

  const busy = new Set(
    service.roles.flatMap((r) =>
      r.slots.filter((s) => s.personId !== 0 && s.assignmentId !== slot.assignmentId).map((s) => s.personId)
    )
  )
  const unavailable = new Set(
    unavailabilities
      .filter((u) => u.startDate <= service.date && service.date <= u.endDate)
      .map((u) => u.personId)
  )
  const withRole = ministry.members.filter((m) => m.roles.some((r) => r.roleId === role.roleId))
  const others = ministry.members.filter((m) => !m.roles.some((r) => r.roleId === role.roleId))

  const option = (member: (typeof ministry.members)[number]) => {
    const current = member.personId === slot.personId
    return (
      <li key={member.personId}>
        <button
          type="button"
          disabled={assign.isPending || current}
          onClick={() => assign.mutate(member.personId)}
          className={cn(
            "flex w-full items-center gap-2 px-3 py-2.5 text-left text-sm transition-colors hover:bg-accent disabled:opacity-100",
            current && "bg-primary/10"
          )}
        >
          <span className="flex-1">{member.name}</span>
          {unavailable.has(member.personId) && <Badge variant="destructive">Não pode</Badge>}
          {busy.has(member.personId) && <Badge variant="secondary">Já escalado</Badge>}
          {current && <Check className="size-4 text-primary" />}
        </button>
      </li>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>{role.name}</DialogTitle>
        <DialogDescription>
          {service.name} · {weekdayOf(service.date)}, {formatDay(service.date)}
        </DialogDescription>
      </DialogHeader>
      <div className="max-h-[50svh] overflow-y-auto">
        <ul className="flex flex-col divide-y rounded-lg border">{withRole.map(option)}</ul>
        {others.length > 0 && (
          <>
            <p className="mt-4 mb-2 text-xs text-muted-foreground">Outras pessoas da equipe</p>
            <ul className="flex flex-col divide-y rounded-lg border">{others.map(option)}</ul>
          </>
        )}
      </div>
      <DialogFooter>
        <Button
          type="button"
          variant="outline"
          disabled={assign.isPending || slot.personId === 0}
          onClick={() => assign.mutate(null)}
        >
          Deixar vago
        </Button>
      </DialogFooter>
    </div>
  )
}

import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Check, Info, Loader2 } from "lucide-react"
import { AnimatePresence, motion } from "motion/react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { ConfirmButton } from "@/components/confirm-button"
import { quick } from "@/components/motion"
import { RoleIcon } from "@/components/role-icon"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import {
  api,
  queryKeys,
  type Member,
  type MinistryDetail,
  type ScheduleRole,
  type ScheduleService,
  type ScheduleSlot,
  type Unavailability,
} from "@/lib/api"
import { toastError } from "@/lib/errors"
import { useRetained } from "@/lib/use-retained"
import { absenceCovers, absenceLabel, dayAndTime, firstName, initials, reasonLabel, todayISO } from "@/lib/format"
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
  published: boolean
  unavailabilities: Unavailability[]
}

export function SlotDialog({ selection, onClose, ministry, month, published, unavailabilities }: Props) {
  const shown = useRetained(selection)
  return (
    <Dialog open={Boolean(selection)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        {shown && (
          <SlotPicker
            key={shown.slot.assignmentId}
            selection={shown}
            ministry={ministry}
            month={month}
            published={published}
            unavailabilities={unavailabilities}
            onDone={onClose}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

/** NOTE: undefined means no change is waiting; null means leaving the slot empty. */
type Pending = { personId: number | null; name: string } | undefined

function SlotPicker({
  selection,
  ministry,
  month,
  published,
  unavailabilities,
  onDone,
}: {
  selection: SlotSelection
  ministry: MinistryDetail
  month: string
  published: boolean
  unavailabilities: Unavailability[]
  onDone: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { service, role, slot } = selection
  const [pending, setPending] = useState<Pending>()
  const past = service.date < todayISO()
  const current = slot.personId !== 0 ? slot.personName : ""

  const assign = useMutation({
    mutationFn: (personId: number | null) => api.schedules.assign(ministry.id, month, slot.assignmentId, personId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.schedule(ministry.id, month) })
      toast.success(t("slot.updated"))
      onDone()
    },
    onError: toastError,
  })

  const decline = useMutation({
    mutationFn: () => api.schedules.decline(ministry.id, month, slot.assignmentId),
    onSuccess: (request) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.schedule(ministry.id, month) })
      if (request.invites.length > 0) {
        toast.success(t("slot.invited", { names: request.invites.map((invite) => firstName(invite.name)).join(", ") }))
      } else {
        toast.warning(t("slot.nobodyFree"))
      }
      onDone()
    },
    onError: toastError,
  })

  // NOTE: Mirrors what the server sends for a change to a published schedule; past services send nothing.
  const changeNote = (change: NonNullable<Pending>) => {
    if (past) {
      return undefined
    }
    const from = firstName(current)
    const to = firstName(change.name)
    const group = ministry.whatsappGroup !== ""
    if (change.personId === null) {
      return current ? t("slot.noteVacate", { from }) : undefined
    }
    if (current) {
      return t(group ? "slot.noteSwapGroup" : "slot.noteSwap", { from, to })
    }
    return t(group ? "slot.noteFillGroup" : "slot.noteFill", { to })
  }

  const choose = (personId: number | null, name: string) => {
    if (published) {
      setPending({ personId, name })
    } else {
      assign.mutate(personId)
    }
  }

  const busy = new Set(
    service.roles.flatMap((r) => r.slots.filter((s) => s.personId !== 0 && s.assignmentId !== slot.assignmentId).map((s) => s.personId))
  )
  const absences = new Map(
    unavailabilities.filter((u) => absenceCovers(u, service.date)).map((u) => [u.personId, u] as const)
  )
  const rank = (member: Member) => (absences.has(member.personId) ? 2 : busy.has(member.personId) ? 1 : 0)
  const byAvailability = (a: Member, b: Member) => rank(a) - rank(b) || a.name.localeCompare(b.name)
  const withRole = ministry.members.filter((m) => m.roles.some((r) => r.roleId === role.roleId)).sort(byAvailability)
  const others = ministry.members.filter((m) => !m.roles.some((r) => r.roleId === role.roleId)).sort(byAvailability)

  const option = (member: Member) => {
    const selected = member.personId === slot.personId
    const absence = absences.get(member.personId)
    return (
      <li key={member.personId}>
        <button
          type="button"
          disabled={assign.isPending || selected}
          aria-current={selected || undefined}
          onClick={() => choose(member.personId, member.name)}
          className={cn(
            "flex min-h-12 w-full items-center gap-3 px-3 py-2 text-left text-sm transition-colors hover:bg-accent/50 focus-visible:bg-accent/50 focus-visible:outline-none disabled:cursor-default",
            selected && "bg-primary/10 hover:bg-primary/10"
          )}
        >
          <span
            className={cn(
              "flex size-8 shrink-0 items-center justify-center rounded-full text-xs font-semibold",
              selected ? "bg-primary text-primary-foreground" : "bg-secondary text-secondary-foreground"
            )}
          >
            {initials(member.name)}
          </span>
          <span className="min-w-0 flex-1">
            <span className={cn("block truncate font-medium", absence && "text-muted-foreground")}>{member.name}</span>
            {absence && (
              <span className="block truncate text-xs text-danger">
                {t("slot.unavailable")} · {absence.fromPoll ? absenceLabel(absence) : `${reasonLabel(absence.reason)}, ${absenceLabel(absence).toLowerCase()}`}
              </span>
            )}
          </span>
          {busy.has(member.personId) && <Badge variant="secondary">{t("slot.busy")}</Badge>}
          {selected && <Check className="size-4 shrink-0 text-brand" aria-hidden />}
        </button>
      </li>
    )
  }

  return (
    <div className="flex min-w-0 flex-col gap-5">
      <DialogHeader>
        <DialogTitle className="flex items-center gap-2">
          <RoleIcon name={role.name} className="size-5 text-brand" aria-hidden />
          {role.name}
        </DialogTitle>
        <DialogDescription>
          {t("slot.when", { service: service.name, day: dayAndTime(service.date, service.startTime) })}
        </DialogDescription>
      </DialogHeader>

      <AnimatePresence mode="wait" initial={false}>
        {pending ? (
          <motion.div
            key="confirm"
            initial={{ opacity: 0, x: 16 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: 16 }}
            transition={quick}
            className="flex flex-col gap-4"
          >
            <p className="font-display text-lg leading-snug font-semibold text-balance">
              {pending.personId === null
                ? t("slot.vacateTitle", { from: firstName(current) })
                : current
                  ? t("slot.swapTitle", { from: firstName(current), to: firstName(pending.name) })
                  : t("slot.fillTitle", { to: firstName(pending.name) })}
            </p>
            {changeNote(pending) && (
              <p className="flex gap-2 rounded-xl bg-muted p-3 text-sm text-pretty text-muted-foreground">
                <Info className="mt-0.5 size-4 shrink-0" aria-hidden />
                {changeNote(pending)}
              </p>
            )}
            <DialogFooter>
              <Button variant="ghost" onClick={() => setPending(undefined)} disabled={assign.isPending}>
                {t("slot.back")}
              </Button>
              <Button
                variant={pending.personId === null ? "destructive" : "default"}
                onClick={() => assign.mutate(pending.personId)}
                disabled={assign.isPending}
              >
                {assign.isPending && <Loader2 className="animate-spin" />}
                {pending.personId === null ? t("slot.confirmVacate") : current ? t("slot.confirmSwap") : t("slot.confirmFill")}
              </Button>
            </DialogFooter>
          </motion.div>
        ) : (
          <motion.div
            key="list"
            initial={{ opacity: 0, x: -16 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -16 }}
            transition={quick}
            className="flex min-w-0 flex-col gap-4"
          >
            <div className="flex flex-col gap-4">
              <section className="flex flex-col gap-2">
                <h3 className="text-xs font-medium text-muted-foreground">{t("slot.roleHolders", { role: role.name.toLowerCase() })}</h3>
                <ul className="flex flex-col divide-y overflow-hidden rounded-xl border">{withRole.map(option)}</ul>
              </section>
              {others.length > 0 && (
                <section className="flex flex-col gap-2">
                  <h3 className="text-xs font-medium text-muted-foreground">{t("slot.others")}</h3>
                  <ul className="flex flex-col divide-y overflow-hidden rounded-xl border">{others.map(option)}</ul>
                </section>
              )}
            </div>
            <DialogFooter className="sm:justify-between">
              {published && !past && slot.personId !== 0 && slot.status !== "declined" ? (
                <ConfirmButton
                  label={t("slot.cannot", { name: firstName(slot.personName) })}
                  title={t("slot.cannotTitle", { name: slot.personName })}
                  description={t("slot.cannotLead")}
                  onConfirm={() => decline.mutate()}
                  pending={decline.isPending}
                />
              ) : (
                <span className="hidden sm:block" />
              )}
              <Button
                type="button"
                variant="outline"
                disabled={assign.isPending || slot.personId === 0}
                onClick={() => choose(null, "")}
              >
                {t("slot.leaveEmpty")}
              </Button>
            </DialogFooter>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

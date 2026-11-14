import { revalidateLogic } from "@tanstack/react-form"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Loader2, Send } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { DateBlock } from "@/components/date-block"
import { useAppForm } from "@/components/form"
import { ErrorState } from "@/components/error-state"
import { RevealItem } from "@/components/motion"
import { RoleIcon } from "@/components/role-icon"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { api, queryKeys, type Unavailability } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { absenceLabel, firstName } from "@/lib/format"

import { AbsenceFields, absenceDefaults, absenceInput, validateAbsence } from "./absence-form"

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  defaultDate: string
  /** NOTE: An absence already registered: the dialog opens straight on offering swaps for it. */
  absence?: Unavailability
}

export function UnavailabilityDialog({ open, onOpenChange, defaultDate, absence }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        {open && <UnavailabilitySteps defaultDate={defaultDate} absence={absence} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function UnavailabilitySteps({ defaultDate, absence, onDone }: { defaultDate: string; absence?: Unavailability; onDone: () => void }) {
  const [saved, setSaved] = useState(absence)
  if (saved) {
    return <OpenCovers absence={saved} onDone={onDone} />
  }
  return <UnavailabilityForm defaultDate={defaultDate} onSaved={(u) => (u.scheduled > 0 ? setSaved(u) : onDone())} />
}

function UnavailabilityForm({ defaultDate, onSaved }: { defaultDate: string; onSaved: (u: Unavailability) => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const people = useQuery({ queryKey: queryKeys.people, queryFn: api.people.list })

  const save = useMutation({
    mutationFn: (input: Parameters<typeof api.unavailabilities.create>[0]) => api.unavailabilities.create(input),
    onSuccess: (saved) => {
      queryClient.invalidateQueries({ queryKey: ["unavailabilities"] })
      queryClient.invalidateQueries({ queryKey: ["schedules"] })
      toast.success(t("absence.saved", { name: firstName(saved.personName), label: absenceLabel(saved).toLowerCase() }))
      onSaved(saved)
    },
    onError: toastError,
  })

  const form = useAppForm({
    defaultValues: absenceDefaults(defaultDate),
    validationLogic: revalidateLogic(),
    validators: { onDynamic: ({ value }) => validateAbsence(value, true) },
    onSubmit: async ({ value }) => {
      await save.mutateAsync({ personId: Number(value.personId), ...absenceInput(value) }).catch(() => undefined)
    },
  })

  const personOptions = (people.data ?? []).map((person) => ({ value: String(person.id), label: person.name }))

  return (
    <form.AppForm>
      <form.Form>
        <DialogHeader>
          <DialogTitle>{t("absence.newTitle")}</DialogTitle>
          <DialogDescription>{t("absence.newLead")}</DialogDescription>
        </DialogHeader>
        <form.AppField name="personId">
          {(field) => (
            <field.SelectField
              label={t("absence.person")}
              placeholder={people.isPending ? t("common.loading") : t("absence.personPlaceholder")}
              disabled={!people.data}
              options={personOptions}
            />
          )}
        </form.AppField>
        <AbsenceFields form={form} />
        <DialogFooter>
          <form.SubmitButton pendingLabel={t("absence.saving")}>{t("common.save")}</form.SubmitButton>
        </DialogFooter>
      </form.Form>
    </form.AppForm>
  )
}

/**
 * NOTE: Offers to find substitutes for the services an absence falls on. With mine, it is the member talking about
 * their own absence on their page; otherwise a leader acting for someone.
 */
export function OpenCovers({ absence, onDone, mine = false }: { absence: Unavailability; onDone: () => void; mine?: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const slots = useQuery({
    queryKey: mine ? queryKeys.myAbsenceSlots(absence.id) : queryKeys.unavailabilityAssignments(absence.id),
    queryFn: () => (mine ? api.mine.absenceSlots(absence.id) : api.unavailabilities.assignments(absence.id)),
  })
  const open = useMutation({
    mutationFn: () => (mine ? api.mine.openCovers(absence.id) : api.unavailabilities.openCovers(absence.id)),
    onSuccess: (requests) => {
      queryClient.invalidateQueries({ queryKey: ["unavailabilities"] })
      queryClient.invalidateQueries({ queryKey: ["schedules"] })
      queryClient.invalidateQueries({ queryKey: ["mine"] })
      toast.success(t("absence.coversSent", { count: requests.length }))
      onDone()
    },
    onError: toastError,
  })

  const count = slots.data?.length ?? absence.scheduled
  const label = absenceLabel(absence).toLowerCase()
  const ministries = new Set(slots.data?.map((slot) => slot.ministryId))

  return (
    <div className="flex flex-col gap-5">
      <DialogHeader>
        <DialogTitle>{t("absence.coversTitle")}</DialogTitle>
        <DialogDescription>
          {mine
            ? t("absence.coversLeadMine", { count, label })
            : t("absence.coversLead", { count, label, name: firstName(absence.personName) })}{" "}
          {t("absence.coversHow")}
        </DialogDescription>
      </DialogHeader>
      {slots.isPending ? (
        <div className="flex flex-col gap-2" role="status" aria-label={t("common.loading")}>
          <Skeleton className="h-14 w-full rounded-xl" />
          <Skeleton className="h-14 w-full rounded-xl" />
        </div>
      ) : slots.isError ? (
        <ErrorState error={slots.error} onRetry={() => void slots.refetch()} retrying={slots.isFetching} />
      ) : count === 0 ? (
        <p className="text-sm text-muted-foreground">{t("absence.coversNone")}</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {slots.data.map((slot, index) => {
            return (
              <RevealItem key={slot.assignmentId} index={index} className="flex items-center gap-3 rounded-xl border p-2 pr-3">
                <DateBlock date={slot.date} startTime={slot.startTime} className="w-12 py-1 [&>span:nth-child(2)]:text-xl" />
                <span className="flex min-w-0 flex-1 items-center gap-2 text-sm">
                  <RoleIcon name={slot.roleName} className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                  <span className="truncate">
                    {slot.roleName}
                    {ministries.size > 1 && <span className="text-muted-foreground"> · {slot.ministryName}</span>}
                  </span>
                </span>
              </RevealItem>
            )
          })}
        </ul>
      )}
      <DialogFooter>
        <Button variant="ghost" onClick={onDone}>
          {t("common.notNow")}
        </Button>
        <Button onClick={() => open.mutate()} disabled={open.isPending || slots.isPending || count === 0}>
          {open.isPending ? <Loader2 className="animate-spin" /> : <Send />}
          {open.isPending ? t("absence.sending") : t("absence.coversTitle")}
        </Button>
      </DialogFooter>
    </div>
  )
}

import { revalidateLogic } from "@tanstack/react-form"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { useAppForm } from "@/components/form"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api, type Unavailability } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { todayISO } from "@/lib/format"
import { AbsenceFields, absenceDefaults, absenceInput, validateAbsence } from "@/features/schedules/absence-form"
import { OpenCovers } from "@/features/schedules/unavailability-dialog"

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  absence?: Unavailability
}

export function MyAbsenceDialog({ open, onOpenChange, absence }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>{open && <Steps absence={absence} onDone={() => onOpenChange(false)} />}</DialogContent>
    </Dialog>
  )
}

function Steps({ absence, onDone }: { absence?: Unavailability; onDone: () => void }) {
  const [saved, setSaved] = useState(absence)
  if (saved) {
    return <OpenCovers absence={saved} onDone={onDone} mine />
  }
  return <MyAbsenceForm onSaved={(u) => (u.scheduled > 0 ? setSaved(u) : onDone())} />
}

function MyAbsenceForm({ onSaved }: { onSaved: (u: Unavailability) => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const save = useMutation({
    mutationFn: (input: ReturnType<typeof absenceInput>) => api.mine.createAbsence(input),
    onSuccess: (saved) => {
      queryClient.invalidateQueries({ queryKey: ["mine"] })
      toast.success(t("absence.savedMine"))
      onSaved(saved)
    },
    onError: toastError,
  })
  const form = useAppForm({
    defaultValues: absenceDefaults(todayISO()),
    validationLogic: revalidateLogic(),
    validators: { onDynamic: ({ value }) => validateAbsence(value, false) },
    onSubmit: async ({ value }) => {
      await save.mutateAsync(absenceInput(value)).catch(() => undefined)
    },
  })

  return (
    <form.AppForm>
      <form.Form>
        <DialogHeader>
          <DialogTitle>{t("absence.mineTitle")}</DialogTitle>
          <DialogDescription>{t("absence.mineLead")}</DialogDescription>
        </DialogHeader>
        <AbsenceFields form={form} />
        <DialogFooter>
          <form.SubmitButton pendingLabel={t("absence.saving")}>{t("common.save")}</form.SubmitButton>
        </DialogFooter>
      </form.Form>
    </form.AppForm>
  )
}

import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api, type Unavailability } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { AbsenceFields, absenceInput, absenceReady, emptyAbsence } from "@/features/schedules/absence-fields"
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

function today(): string {
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
}

function MyAbsenceForm({ onSaved }: { onSaved: (u: Unavailability) => void }) {
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState(() => emptyAbsence(today()))
  const save = useMutation({
    mutationFn: () => api.mine.createAbsence(absenceInput(draft)),
    onSuccess: (saved) => {
      queryClient.invalidateQueries({ queryKey: ["mine"] })
      toast.success("Anotado! 🙏")
      onSaved(saved)
    },
    onError: toastError,
  })

  const submit = (event: FormEvent) => {
    event.preventDefault()
    save.mutate()
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>Quando você não pode?</DialogTitle>
        <DialogDescription>Assim a gente não coloca você na escala nesses dias.</DialogDescription>
      </DialogHeader>
      <AbsenceFields value={draft} onChange={setDraft} />
      <DialogFooter>
        <Button type="submit" disabled={save.isPending || !absenceReady(draft)}>
          {save.isPending ? "Salvando…" : "Salvar"}
        </Button>
      </DialogFooter>
    </form>
  )
}

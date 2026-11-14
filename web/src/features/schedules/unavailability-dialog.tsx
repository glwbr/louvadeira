import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { api, queryKeys, type Unavailability } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { absenceLabel, firstName, formatDay, REASON_LABEL, weekdayOf } from "@/lib/format"

import { AbsenceFields, absenceInput, absenceReady, emptyAbsence } from "./absence-fields"

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

function UnavailabilitySteps({
  defaultDate,
  absence,
  onDone,
}: {
  defaultDate: string
  absence?: Unavailability
  onDone: () => void
}) {
  const [saved, setSaved] = useState(absence)
  if (saved) {
    return <OpenCovers absence={saved} onDone={onDone} />
  }
  return <UnavailabilityForm defaultDate={defaultDate} onSaved={(u) => (u.scheduled > 0 ? setSaved(u) : onDone())} />
}

function UnavailabilityForm({ defaultDate, onSaved }: { defaultDate: string; onSaved: (u: Unavailability) => void }) {
  const queryClient = useQueryClient()
  const people = useQuery({ queryKey: queryKeys.people, queryFn: api.people.list })
  const [personId, setPersonId] = useState("")
  const [draft, setDraft] = useState(() => emptyAbsence(defaultDate))

  const save = useMutation({
    mutationFn: () => api.unavailabilities.create({ personId: Number(personId), ...absenceInput(draft) }),
    onSuccess: (saved) => {
      queryClient.invalidateQueries({ queryKey: ["unavailabilities"] })
      queryClient.invalidateQueries({ queryKey: ["schedules"] })
      toast.success("Indisponibilidade registrada")
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
        <DialogTitle>Quem não pode</DialogTitle>
        <DialogDescription>
          O gerador não coloca a pessoa na escala nesses dias. Se ela já estiver numa escala publicada, dá para pedir
          substituto em seguida.
        </DialogDescription>
      </DialogHeader>
      <div className="grid gap-2">
        <Label>Pessoa</Label>
        <Select value={personId} onValueChange={setPersonId}>
          <SelectTrigger className="w-full">
            <SelectValue placeholder="Escolha uma pessoa" />
          </SelectTrigger>
          <SelectContent>
            {people.data?.map((person) => (
              <SelectItem key={person.id} value={String(person.id)}>
                {person.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <AbsenceFields value={draft} onChange={setDraft} />
      <DialogFooter>
        <Button type="submit" disabled={save.isPending || !personId || !absenceReady(draft)}>
          {save.isPending ? "Salvando…" : "Salvar"}
        </Button>
      </DialogFooter>
    </form>
  )
}

/**
 * NOTE: Offers to find substitutes for the services an absence falls on. With mine, it is the member talking about
 * their own absence on their page; otherwise a leader acting for someone.
 */
export function OpenCovers({ absence, onDone, mine = false }: { absence: Unavailability; onDone: () => void; mine?: boolean }) {
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
      toast.success(
        requests.length === 1 ? "Pronto! Já estamos procurando substituto." : `Pronto! Já estamos procurando substituto para os ${requests.length} cultos.`
      )
      onDone()
    },
    onError: toastError,
  })

  const count = slots.data?.length ?? absence.scheduled
  const services = count === 1 ? "1 culto" : `${count} cultos`
  const ministries = new Set(slots.data?.map((slot) => slot.ministryId))

  return (
    <div className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>Procurar substituto</DialogTitle>
        <DialogDescription>
          {mine ? `Você tem ${services}` : `${firstName(absence.personName)} tem ${services}`} na escala nesse período (
          {absenceLabel(absence).toLowerCase()}). Quem pode substituir recebe uma mensagem no WhatsApp, e quem responder
          primeiro fica com o culto.
        </DialogDescription>
      </DialogHeader>
      {slots.isPending ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-5 w-3/4" />
          <Skeleton className="h-5 w-2/3" />
        </div>
      ) : count === 0 ? (
        <p className="text-sm text-muted-foreground">Nenhum culto pendente: já estamos procurando substituto.</p>
      ) : (
        <ul className="flex flex-col gap-1.5 text-sm">
          {slots.data?.map((slot) => (
            <li key={slot.assignmentId} className="flex items-baseline justify-between gap-3">
              <span>
                {weekdayOf(slot.date)}, {formatDay(slot.date)} · {slot.startTime.replace(":", "h")}
              </span>
              <span className="text-muted-foreground">
                {slot.roleName}
                {ministries.size > 1 && ` · ${slot.ministryName}`}
              </span>
            </li>
          ))}
        </ul>
      )}
      <p className="text-xs text-muted-foreground">
        O motivo ({REASON_LABEL[absence.reason].toLowerCase()}) fica só com a liderança.
      </p>
      <DialogFooter>
        <Button variant="outline" onClick={onDone}>
          Agora não
        </Button>
        <Button onClick={() => open.mutate()} disabled={open.isPending || slots.isPending || count === 0}>
          {open.isPending ? "Enviando…" : "Procurar substituto"}
        </Button>
      </DialogFooter>
    </div>
  )
}

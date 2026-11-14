import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { api, queryKeys, type Reason, type Unavailability } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { firstName, formatDateRange, formatDay, REASON_LABEL, weekdayOf } from "@/lib/format"

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
  const [startDate, setStartDate] = useState(defaultDate)
  const [endDate, setEndDate] = useState(defaultDate)
  const [reason, setReason] = useState<Reason | "">("")

  const save = useMutation({
    mutationFn: () => api.unavailabilities.create({ personId: Number(personId), startDate, endDate, reason: reason as Reason }),
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
          O gerador não escala a pessoa nesses dias. Se ela já estiver numa escala publicada, você pode abrir trocas em
          seguida.
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
      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-2">
          <Label htmlFor="unavailable-from">De</Label>
          <Input
            id="unavailable-from"
            type="date"
            value={startDate}
            onChange={(e) => {
              setStartDate(e.target.value)
              if (endDate < e.target.value) setEndDate(e.target.value)
            }}
            required
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="unavailable-to">Até</Label>
          <Input
            id="unavailable-to"
            type="date"
            min={startDate}
            value={endDate}
            onChange={(e) => setEndDate(e.target.value)}
            required
          />
        </div>
      </div>
      <div className="grid gap-2">
        <Label>Motivo</Label>
        <Select value={reason} onValueChange={(value) => setReason(value as Reason)}>
          <SelectTrigger className="w-full">
            <SelectValue placeholder="Escolha o motivo" />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(REASON_LABEL).map(([value, label]) => (
              <SelectItem key={value} value={value}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <DialogFooter>
        <Button type="submit" disabled={save.isPending || !personId || !reason}>
          {save.isPending ? "Salvando…" : "Salvar"}
        </Button>
      </DialogFooter>
    </form>
  )
}

function OpenCovers({ absence, onDone }: { absence: Unavailability; onDone: () => void }) {
  const queryClient = useQueryClient()
  const slots = useQuery({
    queryKey: queryKeys.unavailabilityAssignments(absence.id),
    queryFn: () => api.unavailabilities.assignments(absence.id),
  })
  const open = useMutation({
    mutationFn: () => api.unavailabilities.openCovers(absence.id),
    onSuccess: (requests) => {
      queryClient.invalidateQueries({ queryKey: ["unavailabilities"] })
      queryClient.invalidateQueries({ queryKey: ["schedules"] })
      toast.success(requests.length === 1 ? "Troca aberta" : `${requests.length} trocas abertas`)
      onDone()
    },
    onError: toastError,
  })

  const count = slots.data?.length ?? absence.scheduled
  const ministries = new Set(slots.data?.map((slot) => slot.ministryId))

  return (
    <div className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>Abrir trocas</DialogTitle>
        <DialogDescription>
          {firstName(absence.personName)} tem {count === 1 ? "1 culto" : `${count} cultos`} na escala de{" "}
          {formatDateRange(absence.startDate, absence.endDate)}. Quem pode cobrir recebe uma enquete no WhatsApp com os
          cultos que pode pegar, e quem marcar primeiro fica com o culto.
        </DialogDescription>
      </DialogHeader>
      {slots.isPending ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-5 w-3/4" />
          <Skeleton className="h-5 w-2/3" />
        </div>
      ) : count === 0 ? (
        <p className="text-sm text-muted-foreground">Nenhum culto para trocar: as trocas já foram abertas.</p>
      ) : (
        <ul className="flex flex-col gap-1.5 text-sm">
          {slots.data?.map((slot) => (
            <li key={slot.assignmentId} className="flex items-baseline justify-between gap-3">
              <span>
                {weekdayOf(slot.date)} {formatDay(slot.date)} · {slot.startTime.replace(":", "h")}
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
        O motivo ({REASON_LABEL[absence.reason].toLowerCase()}) só aparece para a liderança.
      </p>
      <DialogFooter>
        <Button variant="outline" onClick={onDone}>
          Agora não
        </Button>
        <Button onClick={() => open.mutate()} disabled={open.isPending || slots.isPending || count === 0}>
          {open.isPending ? "Abrindo…" : count === 1 ? "Abrir troca" : `Abrir trocas para os ${count}`}
        </Button>
      </DialogFooter>
    </div>
  )
}

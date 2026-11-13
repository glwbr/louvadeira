import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { api, queryKeys } from "@/lib/api"
import { toastError } from "@/lib/errors"

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  defaultDate: string
}

export function UnavailabilityDialog({ open, onOpenChange, defaultDate }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        {open && <UnavailabilityForm defaultDate={defaultDate} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function UnavailabilityForm({ defaultDate, onDone }: { defaultDate: string; onDone: () => void }) {
  const queryClient = useQueryClient()
  const people = useQuery({ queryKey: queryKeys.people, queryFn: api.people.list })
  const [personId, setPersonId] = useState("")
  const [startDate, setStartDate] = useState(defaultDate)
  const [endDate, setEndDate] = useState(defaultDate)
  const [note, setNote] = useState("")

  const save = useMutation({
    mutationFn: () => api.unavailabilities.create({ personId: Number(personId), startDate, endDate, note }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["unavailabilities"] })
      queryClient.invalidateQueries({ queryKey: ["schedules"] })
      toast.success("Indisponibilidade registrada")
      onDone()
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
        <DialogDescription>O gerador não escala a pessoa nesses dias.</DialogDescription>
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
        <Label htmlFor="unavailable-note">Motivo (opcional)</Label>
        <Input id="unavailable-note" value={note} onChange={(e) => setNote(e.target.value)} placeholder="Viagem" />
      </div>
      <DialogFooter>
        <Button type="submit" disabled={save.isPending || !personId}>
          {save.isPending ? "Salvando…" : "Salvar"}
        </Button>
      </DialogFooter>
    </form>
  )
}

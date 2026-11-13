import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ConfirmButton } from "@/components/confirm-button"
import { api, queryKeys, type Person } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { formatPhone } from "@/lib/format"

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  person?: Person
  canArchive?: boolean
}

export function PersonDialog({ open, onOpenChange, person, canArchive }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        {open && <PersonForm person={person} canArchive={canArchive} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function PersonForm({ person, canArchive, onDone }: { person?: Person; canArchive?: boolean; onDone: () => void }) {
  const queryClient = useQueryClient()
  const [name, setName] = useState(person?.name ?? "")
  const [phone, setPhone] = useState(person ? formatPhone(person.phone) : "")

  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.people })
  const save = useMutation({
    mutationFn: () => (person ? api.people.update(person.id, { name, phone }) : api.people.create({ name, phone })),
    onSuccess: () => {
      refresh()
      queryClient.invalidateQueries({ queryKey: queryKeys.ministries })
      toast.success(person ? "Pessoa atualizada" : "Pessoa cadastrada")
      onDone()
    },
    onError: toastError,
  })
  const archive = useMutation({
    mutationFn: () => api.people.archive(person!.id),
    onSuccess: () => {
      refresh()
      queryClient.invalidateQueries({ queryKey: queryKeys.ministries })
      toast.success("Pessoa arquivada")
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
        <DialogTitle>{person ? "Editar pessoa" : "Nova pessoa"}</DialogTitle>
        <DialogDescription>O telefone é o WhatsApp que recebe as escalas.</DialogDescription>
      </DialogHeader>
      <div className="grid gap-2">
        <Label htmlFor="person-name">Nome</Label>
        <Input id="person-name" value={name} onChange={(e) => setName(e.target.value)} autoFocus required />
      </div>
      <div className="grid gap-2">
        <Label htmlFor="person-phone">WhatsApp</Label>
        <Input
          id="person-phone"
          type="tel"
          inputMode="tel"
          placeholder="(11) 98888-7777"
          value={phone}
          onChange={(e) => setPhone(e.target.value)}
        />
      </div>
      <DialogFooter className="gap-2 sm:justify-between">
        {person && canArchive ? (
          <ConfirmButton
            label="Arquivar"
            title={`Arquivar ${person.name}?`}
            description="A pessoa sai de todos os ministérios e não aparece mais nas listas."
            onConfirm={() => archive.mutate()}
            pending={archive.isPending}
          />
        ) : (
          <span />
        )}
        <Button type="submit" disabled={save.isPending}>
          {save.isPending ? "Salvando…" : "Salvar"}
        </Button>
      </DialogFooter>
    </form>
  )
}

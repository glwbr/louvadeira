import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { ConfirmButton } from "@/components/confirm-button"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api, queryKeys, type Role } from "@/lib/api"
import { toastError } from "@/lib/errors"

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  ministryId: number
  role?: Role
}

export function RoleDialog({ open, onOpenChange, ministryId, role }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        {open && <RoleForm ministryId={ministryId} role={role} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function RoleForm({ ministryId, role, onDone }: { ministryId: number; role?: Role; onDone: () => void }) {
  const queryClient = useQueryClient()
  const [name, setName] = useState(role?.name ?? "")
  const [slots, setSlots] = useState(String(role?.slots ?? 1))

  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.ministry(ministryId) })
  const save = useMutation({
    mutationFn: async () => {
      const input = { name, slots: Number(slots) }
      if (role) {
        await api.ministries.updateRole(ministryId, role.id, input)
      } else {
        await api.ministries.createRole(ministryId, input)
      }
    },
    onSuccess: () => {
      refresh()
      toast.success(role ? "Função atualizada" : "Função criada")
      onDone()
    },
    onError: toastError,
  })
  const remove = useMutation({
    mutationFn: () => api.ministries.deleteRole(ministryId, role!.id),
    onSuccess: () => {
      refresh()
      toast.success("Função removida")
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
        <DialogTitle>{role ? "Editar função" : "Nova função"}</DialogTitle>
        <DialogDescription>Voz, baixo, bateria, violão…</DialogDescription>
      </DialogHeader>
      <div className="grid gap-2">
        <Label htmlFor="role-name">Nome</Label>
        <Input id="role-name" value={name} onChange={(e) => setName(e.target.value)} autoFocus required />
      </div>
      <div className="grid gap-2">
        <Label htmlFor="role-slots">Vagas por escala</Label>
        <Input
          id="role-slots"
          type="number"
          inputMode="numeric"
          min={1}
          max={20}
          value={slots}
          onChange={(e) => setSlots(e.target.value)}
          required
        />
        <p className="text-xs text-muted-foreground">
          Quantas pessoas fazem essa função em cada escala, contando quem é fixo. Ex.: 3 vozes.
        </p>
      </div>
      <DialogFooter className="gap-2 sm:justify-between">
        {role ? (
          <ConfirmButton
            label="Remover"
            title={`Remover a função ${role.name}?`}
            description="Quem faz essa função perde o vínculo com ela neste ministério."
            onConfirm={() => remove.mutate()}
            pending={remove.isPending}
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

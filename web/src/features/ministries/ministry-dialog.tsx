import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState, type FormEvent } from "react"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { ConfirmButton } from "@/components/confirm-button"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { api, queryKeys, type Ministry, type RotationUnit } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { rotationLabel } from "@/lib/format"

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  ministry?: Ministry
}

export function MinistryDialog({ open, onOpenChange, ministry }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>{open && <MinistryForm ministry={ministry} onDone={() => onOpenChange(false)} />}</DialogContent>
    </Dialog>
  )
}

function MinistryForm({ ministry, onDone }: { ministry?: Ministry; onDone: () => void }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [name, setName] = useState(ministry?.name ?? "")
  const [rotationUnit, setRotationUnit] = useState<RotationUnit>(ministry?.rotationUnit ?? "week")
  const [whatsappGroup, setWhatsappGroup] = useState(ministry?.whatsappGroup || "none")
  const groups = useQuery({ queryKey: queryKeys.whatsappGroups, queryFn: api.whatsapp.groups, retry: false, staleTime: 5 * 60_000 })
  const group = whatsappGroup === "none" ? "" : whatsappGroup

  const save = useMutation({
    mutationFn: async () => {
      if (ministry) {
        await api.ministries.update(ministry.id, { name, rotationUnit, whatsappGroup: group })
        return ministry.id
      }
      return (await api.ministries.create({ name, rotationUnit, whatsappGroup: group })).id
    },
    onSuccess: (id) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.ministries })
      toast.success(ministry ? "Ministério atualizado" : "Ministério criado")
      onDone()
      if (!ministry) {
        navigate(`/ministerios/${id}`)
      }
    },
    onError: toastError,
  })
  const archive = useMutation({
    mutationFn: () => api.ministries.archive(ministry!.id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.ministries })
      toast.success("Ministério arquivado")
      onDone()
      navigate("/ministerios")
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
        <DialogTitle>{ministry ? "Editar ministério" : "Novo ministério"}</DialogTitle>
        <DialogDescription>Louvor, dança, mídia, diáconos…</DialogDescription>
      </DialogHeader>
      <div className="grid gap-2">
        <Label htmlFor="ministry-name">Nome</Label>
        <Input id="ministry-name" value={name} onChange={(e) => setName(e.target.value)} autoFocus required />
      </div>
      <div className="grid gap-2">
        <Label>Rodízio</Label>
        <Select value={rotationUnit} onValueChange={(value) => setRotationUnit(value as RotationUnit)}>
          <SelectTrigger className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="week">{rotationLabel("week")}</SelectItem>
            <SelectItem value="service">{rotationLabel("service")}</SelectItem>
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-foreground">
          Por semana: a mesma equipe serve em todos os cultos da semana. Por culto: cada culto tem a sua equipe.
        </p>
      </div>
      <div className="grid gap-2">
        <Label>Grupo do WhatsApp</Label>
        <Select value={whatsappGroup} onValueChange={setWhatsappGroup} disabled={!groups.data}>
          <SelectTrigger className="w-full">
            <SelectValue placeholder={groups.isError ? "WhatsApp desconectado" : "Carregando grupos…"} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="none">Sem grupo</SelectItem>
            {groups.data?.map((g) => (
              <SelectItem key={g.id} value={g.id}>
                {g.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-foreground">Onde a escala publicada e as trocas são anunciadas.</p>
      </div>
      <DialogFooter className="gap-2 sm:justify-between">
        {ministry ? (
          <ConfirmButton
            label="Arquivar"
            title={`Arquivar ${ministry.name}?`}
            description="O ministério some das listas. As pessoas continuam cadastradas."
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

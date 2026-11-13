import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState, type FormEvent } from "react"
import { Link } from "react-router"
import { toast } from "sonner"

import { ConfirmButton } from "@/components/confirm-button"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { api, queryKeys, type Member, type MemberRole, type MinistryDetail, type Person } from "@/lib/api"
import { toastError } from "@/lib/errors"

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  ministry: MinistryDetail
  member?: Member
  people: Person[]
}

export function MemberDialog({ open, onOpenChange, ministry, member, people }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        {open && <MemberForm ministry={ministry} member={member} people={people} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function MemberForm({
  ministry,
  member,
  people,
  onDone,
}: {
  ministry: MinistryDetail
  member?: Member
  people: Person[]
  onDone: () => void
}) {
  const queryClient = useQueryClient()
  const memberIds = new Set(ministry.members.map((m) => m.personId))
  const candidates = people.filter((p) => !memberIds.has(p.id))
  const [personId, setPersonId] = useState(member ? String(member.personId) : "")
  const [isLeader, setIsLeader] = useState(member?.isLeader ?? false)
  const [roles, setRoles] = useState<Map<number, MemberRole>>(
    () => new Map((member?.roles ?? []).map((role) => [role.roleId, role]))
  )

  const toggleRole = (roleId: number, selected: boolean) => {
    setRoles((current) => {
      const next = new Map(current)
      if (selected) {
        next.set(roleId, { roleId, fixed: false })
      } else {
        next.delete(roleId)
      }
      return next
    })
  }
  const toggleFixed = (roleId: number, fixed: boolean) => {
    setRoles((current) => new Map(current).set(roleId, { roleId, fixed }))
  }

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.ministry(ministry.id) })
    queryClient.invalidateQueries({ queryKey: queryKeys.ministries })
  }
  const save = useMutation({
    mutationFn: () => api.ministries.putMember(ministry.id, Number(personId), { isLeader, roles: [...roles.values()] }),
    onSuccess: () => {
      refresh()
      toast.success(member ? "Equipe atualizada" : "Pessoa adicionada à equipe")
      onDone()
    },
    onError: toastError,
  })
  const remove = useMutation({
    mutationFn: () => api.ministries.removeMember(ministry.id, member!.personId),
    onSuccess: () => {
      refresh()
      toast.success("Pessoa removida da equipe")
      onDone()
    },
    onError: toastError,
  })

  const submit = (event: FormEvent) => {
    event.preventDefault()
    save.mutate()
  }

  if (!member && candidates.length === 0) {
    return (
      <DialogHeader>
        <DialogTitle>Todo mundo já está na equipe</DialogTitle>
        <DialogDescription>
          Para adicionar alguém novo, cadastre a pessoa em <Link to="/pessoas" className="text-primary underline">Pessoas</Link>{" "}
          primeiro.
        </DialogDescription>
      </DialogHeader>
    )
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>{member ? member.name : "Adicionar à equipe"}</DialogTitle>
        <DialogDescription>Marque o que a pessoa faz. Fixo é quem entra em toda escala.</DialogDescription>
      </DialogHeader>

      {!member && (
        <div className="grid gap-2">
          <Label>Pessoa</Label>
          <Select value={personId} onValueChange={setPersonId} required>
            <SelectTrigger className="w-full">
              <SelectValue placeholder="Escolha uma pessoa" />
            </SelectTrigger>
            <SelectContent>
              {candidates.map((person) => (
                <SelectItem key={person.id} value={String(person.id)}>
                  {person.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}

      <div className="grid gap-2">
        <Label>Funções</Label>
        <ul className="flex flex-col divide-y rounded-lg border">
          {ministry.roles.map((role) => {
            const selected = roles.get(role.id)
            const checkboxId = `role-${role.id}`
            return (
              <li key={role.id} className="flex items-center gap-3 px-3 py-2.5">
                <Checkbox
                  id={checkboxId}
                  checked={Boolean(selected)}
                  onCheckedChange={(checked) => toggleRole(role.id, checked === true)}
                />
                <label htmlFor={checkboxId} className="flex-1 text-sm">
                  {role.name}
                </label>
                {selected && (
                  <label className="flex items-center gap-2 text-xs text-muted-foreground">
                    Fixo
                    <Switch checked={selected.fixed} onCheckedChange={(fixed) => toggleFixed(role.id, fixed)} />
                  </label>
                )}
              </li>
            )
          })}
        </ul>
      </div>

      <label className="flex items-center justify-between gap-3 rounded-lg border p-3">
        <div>
          <p className="text-sm font-medium">Líder do ministério</p>
          <p className="text-xs text-muted-foreground">Recebe os avisos de trocas e faltas.</p>
        </div>
        <Switch checked={isLeader} onCheckedChange={setIsLeader} />
      </label>

      <DialogFooter className="gap-2 sm:justify-between">
        {member ? (
          <ConfirmButton
            label="Tirar da equipe"
            title={`Tirar ${member.name} da equipe?`}
            description="A pessoa continua cadastrada e pode voltar depois."
            onConfirm={() => remove.mutate()}
            pending={remove.isPending}
          />
        ) : (
          <span />
        )}
        <Button type="submit" disabled={save.isPending || !personId}>
          {save.isPending ? "Salvando…" : "Salvar"}
        </Button>
      </DialogFooter>
    </form>
  )
}

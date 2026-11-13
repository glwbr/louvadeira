import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ArrowDown, ArrowLeft, ArrowUp, CalendarClock, Crown, ListChecks, Pencil, Pin, Plus, Users } from "lucide-react"
import { useState } from "react"
import { Link, useParams } from "react-router"

import { EmptyState } from "@/components/empty-state"
import { ListSkeleton } from "@/components/list-skeleton"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { api, queryKeys, type Member, type MinistryDetail, type Role } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { ROTATION_LABEL, serviceLabel } from "@/lib/format"
import { MemberDialog } from "./member-dialog"
import { MinistryDialog } from "./ministry-dialog"
import { RoleDialog } from "./role-dialog"

export function MinistryPage() {
  const id = Number(useParams().id)
  const ministry = useQuery({ queryKey: queryKeys.ministry(id), queryFn: () => api.ministries.get(id) })
  const [editing, setEditing] = useState(false)

  if (ministry.isPending) {
    return <ListSkeleton rows={4} />
  }
  if (ministry.isError) {
    return (
      <EmptyState icon={ListChecks} title="Ministério não encontrado">
        <Link to="/ministerios" className="text-primary underline">
          Voltar para os ministérios
        </Link>
      </EmptyState>
    )
  }

  const detail = ministry.data
  return (
    <>
      <Link to="/ministerios" className="mb-3 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" />
        Ministérios
      </Link>
      <div className="mb-5 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="truncate text-2xl font-semibold tracking-tight">{detail.name}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{ROTATION_LABEL[detail.rotationUnit]}</p>
        </div>
        <Button variant="outline" size="sm" onClick={() => setEditing(true)}>
          <Pencil />
          Editar
        </Button>
      </div>

      <Tabs defaultValue={detail.roles.length > 0 ? "equipe" : "funcoes"}>
        <TabsList className="mb-4 w-full">
          <TabsTrigger value="equipe">Equipe ({detail.members.length})</TabsTrigger>
          <TabsTrigger value="funcoes">Funções ({detail.roles.length})</TabsTrigger>
          <TabsTrigger value="cultos">Cultos ({detail.serviceIds.length})</TabsTrigger>
        </TabsList>
        <TabsContent value="equipe">
          <TeamTab ministry={detail} />
        </TabsContent>
        <TabsContent value="funcoes">
          <RolesTab ministry={detail} />
        </TabsContent>
        <TabsContent value="cultos">
          <ServicesTab ministry={detail} />
        </TabsContent>
      </Tabs>

      <MinistryDialog open={editing} onOpenChange={setEditing} ministry={detail} />
    </>
  )
}

function TeamTab({ ministry }: { ministry: MinistryDetail }) {
  const people = useQuery({ queryKey: queryKeys.people, queryFn: api.people.list })
  const [selected, setSelected] = useState<Member | undefined>()
  const [open, setOpen] = useState(false)
  const roleNames = new Map(ministry.roles.map((role) => [role.id, role.name]))

  const openDialog = (member?: Member) => {
    setSelected(member)
    setOpen(true)
  }

  if (ministry.roles.length === 0) {
    return (
      <EmptyState icon={ListChecks} title="Crie as funções primeiro">
        Na aba Funções, cadastre o que a equipe faz (voz, baixo, bateria…). Depois volte aqui para montar a equipe.
      </EmptyState>
    )
  }

  return (
    <>
      <div className="mb-3 flex justify-end">
        <Button size="sm" onClick={() => openDialog()} disabled={!people.data}>
          <Plus />
          Adicionar pessoa
        </Button>
      </div>
      {ministry.members.length === 0 ? (
        <EmptyState icon={Users} title="Equipe vazia">
          Adicione as pessoas e marque o que cada uma faz.
        </EmptyState>
      ) : (
        <ul className="flex flex-col divide-y rounded-xl border bg-card">
          {ministry.members.map((member) => (
            <li key={member.personId}>
              <button
                type="button"
                onClick={() => openDialog(member)}
                className="flex w-full flex-col gap-2 px-4 py-3 text-left transition-colors hover:bg-accent"
              >
                <div className="flex items-center gap-2">
                  <span className="font-medium">{member.name}</span>
                  {member.isLeader && (
                    <Badge variant="outline">
                      <Crown />
                      Líder
                    </Badge>
                  )}
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {member.roles.length === 0 && <span className="text-sm text-muted-foreground">Sem função</span>}
                  {member.roles.map((role) => (
                    <Badge key={role.roleId} variant={role.fixed ? "default" : "secondary"}>
                      {role.fixed && <Pin />}
                      {roleNames.get(role.roleId)}
                    </Badge>
                  ))}
                </div>
              </button>
            </li>
          ))}
        </ul>
      )}
      {people.data && (
        <MemberDialog open={open} onOpenChange={setOpen} ministry={ministry} member={selected} people={people.data} />
      )}
    </>
  )
}

function RolesTab({ ministry }: { ministry: MinistryDetail }) {
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<Role | undefined>()
  const [open, setOpen] = useState(false)
  const reorder = useMutation({
    mutationFn: (roleIds: number[]) => api.ministries.reorderRoles(ministry.id, roleIds),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.ministry(ministry.id) }),
    onError: toastError,
  })
  const move = (index: number, delta: number) => {
    const ids = ministry.roles.map((role) => role.id)
    ;[ids[index], ids[index + delta]] = [ids[index + delta], ids[index]]
    reorder.mutate(ids)
  }

  const openDialog = (role?: Role) => {
    setSelected(role)
    setOpen(true)
  }
  const holders = (roleId: number) =>
    ministry.members.filter((member) => member.roles.some((role) => role.roleId === roleId)).length

  return (
    <>
      <div className="mb-3 flex justify-end">
        <Button size="sm" onClick={() => openDialog()}>
          <Plus />
          Nova função
        </Button>
      </div>
      {ministry.roles.length === 0 ? (
        <EmptyState icon={ListChecks} title="Nenhuma função">
          Ex.: Voz com 3 vagas, Violão com 1, Baixo com 1, Bateria com 1.
        </EmptyState>
      ) : (
        <>
          <p className="mb-3 text-sm text-muted-foreground">
            A ordem é a prioridade quando falta gente: as primeiras funções são preenchidas antes. Deixe o violão antes
            da bateria para o baterista cobrir o violão quando o líder faltar.
          </p>
          <ul className="flex flex-col divide-y rounded-xl border bg-card">
            {ministry.roles.map((role, index) => (
              <li key={role.id} className="flex items-center gap-1 pr-2">
                <button
                  type="button"
                  onClick={() => openDialog(role)}
                  className="flex min-w-0 flex-1 items-center justify-between gap-3 px-4 py-3 text-left transition-colors hover:bg-accent"
                >
                  <div className="min-w-0">
                    <p className="font-medium">
                      <span className="mr-2 text-muted-foreground">{index + 1}.</span>
                      {role.name}
                    </p>
                    <p className="text-sm text-muted-foreground">
                      {role.slots} {role.slots === 1 ? "vaga" : "vagas"} por escala · {holders(role.id)}{" "}
                      {holders(role.id) === 1 ? "pessoa faz" : "pessoas fazem"}
                    </p>
                  </div>
                  <Pencil className="size-4 shrink-0 text-muted-foreground" />
                </button>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={`Subir ${role.name}`}
                  disabled={index === 0 || reorder.isPending}
                  onClick={() => move(index, -1)}
                >
                  <ArrowUp />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={`Descer ${role.name}`}
                  disabled={index === ministry.roles.length - 1 || reorder.isPending}
                  onClick={() => move(index, 1)}
                >
                  <ArrowDown />
                </Button>
              </li>
            ))}
          </ul>
        </>
      )}
      <RoleDialog open={open} onOpenChange={setOpen} ministryId={ministry.id} role={selected} />
    </>
  )
}

function ServicesTab({ ministry }: { ministry: MinistryDetail }) {
  const queryClient = useQueryClient()
  const services = useQuery({ queryKey: queryKeys.services, queryFn: api.services.list })
  const save = useMutation({
    mutationFn: (serviceIds: number[]) => api.ministries.setServices(ministry.id, serviceIds),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.ministry(ministry.id) }),
    onError: toastError,
  })
  const chosen = new Set(ministry.serviceIds)

  const toggle = (serviceId: number, checked: boolean) => {
    const next = new Set(chosen)
    if (checked) {
      next.add(serviceId)
    } else {
      next.delete(serviceId)
    }
    save.mutate([...next])
  }

  const active = services.data?.filter((service) => service.active || chosen.has(service.id)) ?? []
  if (services.isPending) {
    return <ListSkeleton rows={2} />
  }
  if (active.length === 0) {
    return (
      <EmptyState icon={CalendarClock} title="Nenhum culto cadastrado">
        Cadastre os cultos em{" "}
        <Link to="/cultos" className="text-primary underline">
          Cultos
        </Link>{" "}
        e depois marque aqui em quais este ministério serve.
      </EmptyState>
    )
  }

  return (
    <>
      <p className="mb-3 text-sm text-muted-foreground">Marque os cultos em que este ministério tem escala.</p>
      <ul className="flex flex-col divide-y rounded-xl border bg-card">
        {active.map((service) => {
          const checkboxId = `service-${service.id}`
          return (
            <li key={service.id} className="flex items-center gap-3 px-4 py-3">
              <Checkbox
                id={checkboxId}
                checked={chosen.has(service.id)}
                disabled={save.isPending}
                onCheckedChange={(checked) => toggle(service.id, checked === true)}
              />
              <label htmlFor={checkboxId} className="flex-1">
                <p className="font-medium">{service.name}</p>
                <p className="text-sm text-muted-foreground">{serviceLabel(service)}</p>
              </label>
            </li>
          )
        })}
      </ul>
    </>
  )
}

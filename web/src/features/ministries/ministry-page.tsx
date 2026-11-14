import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ArrowDown, ArrowLeft, ArrowUp, CalendarClock, Crown, ListChecks, Pencil, Pin, Plus, Users } from "lucide-react"
import { motion } from "motion/react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useParams } from "react-router"
import { toast } from "sonner"

import { EmptyState } from "@/components/empty-state"
import { ErrorState } from "@/components/error-state"
import { ListSkeleton } from "@/components/list-skeleton"
import { Avatar } from "@/components/avatar"
import { ListCard, ListRow } from "@/components/list"
import { AnimatedItem, spring } from "@/components/motion"
import { RoleIcon } from "@/components/role-icon"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ApiError, api, queryKeys, type Member, type MinistryDetail, type Role } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { communionLabel, rotationLabel, serviceLabel } from "@/lib/format"
import { cn } from "@/lib/utils"

import { MemberDialog } from "./member-dialog"
import { MinistryDialog } from "./ministry-dialog"
import { RoleDialog } from "./role-dialog"

export function MinistryPage() {
  const { t } = useTranslation()
  const id = Number(useParams().id)
  const ministry = useQuery({ queryKey: queryKeys.ministry(id), queryFn: () => api.ministries.get(id) })
  const [editing, setEditing] = useState(false)

  const back = (
    <Link
      to="/ministerios"
      className="-ml-2 mb-3 inline-flex h-10 items-center gap-1.5 rounded-lg px-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
    >
      <ArrowLeft className="size-4" />
      {t("ministries.back")}
    </Link>
  )

  if (ministry.isPending) {
    return (
      <>
        {back}
        <Skeleton className="mb-2 h-9 w-48" />
        <Skeleton className="mb-6 h-4 w-32" />
        <Skeleton className="mb-4 h-11 w-full rounded-xl" />
        <ListSkeleton rows={4} />
      </>
    )
  }
  if (ministry.isError) {
    const missing = ministry.error instanceof ApiError && ministry.error.status === 404
    return (
      <>
        {back}
        {missing ? (
          <EmptyState icon={ListChecks} title={t("ministries.notFoundTitle")} />
        ) : (
          <ErrorState error={ministry.error} onRetry={() => void ministry.refetch()} retrying={ministry.isFetching} />
        )}
      </>
    )
  }

  const detail = ministry.data
  return (
    <>
      {back}
      <div className="mb-6 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="font-display text-[2rem] leading-[1.1] font-semibold text-balance">{detail.name}</h1>
          <p className="mt-1.5 text-sm text-muted-foreground">{rotationLabel(detail.rotationUnit)}</p>
        </div>
        <Button variant="outline" size="sm" className="mt-1 shrink-0" onClick={() => setEditing(true)}>
          <Pencil />
          {t("ministries.edit")}
        </Button>
      </div>

      <Tabs defaultValue={detail.roles.length > 0 ? "team" : "roles"}>
        <TabsList className="mb-5 w-full">
          <TabsTrigger value="team">
            {t("ministries.tabs.team")} <Count value={detail.members.length} />
          </TabsTrigger>
          <TabsTrigger value="roles">
            {t("ministries.tabs.roles")} <Count value={detail.roles.length} />
          </TabsTrigger>
          <TabsTrigger value="services">
            {t("ministries.tabs.services")} <Count value={detail.serviceIds.length} />
          </TabsTrigger>
        </TabsList>
        <TabsContent value="team">
          <TeamTab ministry={detail} />
        </TabsContent>
        <TabsContent value="roles">
          <RolesTab ministry={detail} />
        </TabsContent>
        <TabsContent value="services">
          <ServicesTab ministry={detail} />
        </TabsContent>
      </Tabs>

      <MinistryDialog open={editing} onOpenChange={setEditing} ministry={detail} />
    </>
  )
}

function Count({ value }: { value: number }) {
  return <span className="tabular text-xs text-muted-foreground">{value}</span>
}

function TeamTab({ ministry }: { ministry: MinistryDetail }) {
  const { t } = useTranslation()
  const people = useQuery({ queryKey: queryKeys.people, queryFn: api.people.list })
  const [selected, setSelected] = useState<Member | undefined>()
  const [open, setOpen] = useState(false)
  const roleNames = new Map(ministry.roles.map((role) => [role.id, role.name]))
  const hasFixed = ministry.members.some((member) => member.roles.some((role) => role.fixed))

  const openDialog = (member?: Member) => {
    setSelected(member)
    setOpen(true)
  }

  if (ministry.roles.length === 0) {
    return (
      <EmptyState icon={ListChecks} title={t("ministries.team.needRolesTitle")}>
        {t("ministries.team.needRoles")}
      </EmptyState>
    )
  }

  return (
    <>
      <div className="mb-3 flex items-center justify-between gap-3">
        {hasFixed ? (
          <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Pin className="size-3.5" aria-hidden />
            {t("ministries.team.fixedLegend")}
          </p>
        ) : (
          <span />
        )}
        <Button size="sm" onClick={() => openDialog()} disabled={!people.data}>
          <Plus />
          {t("ministries.team.add")}
        </Button>
      </div>
      {ministry.members.length === 0 ? (
        <EmptyState icon={Users} title={t("ministries.team.emptyTitle")}>
          {t("ministries.team.empty")}
        </EmptyState>
      ) : (
        <ListCard>
          {ministry.members.map((member) => (
            <AnimatedItem key={member.personId}>
              <ListRow
                onClick={() => openDialog(member)}
                className="items-start py-3.5 [&>svg:last-child]:mt-3"
                leading={<Avatar name={member.name} />}
                title={
                  <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
                    {member.name}
                    {member.isLeader && (
                      <Badge variant="brand">
                        <Crown />
                        {t("ministries.team.leader")}
                      </Badge>
                    )}
                  </span>
                }
                subtitle={
                  <span className="mt-2 flex flex-wrap gap-1.5">
                    {member.roles.length === 0 && t("ministries.team.noRole")}
                    {member.roles.map((role) => {
                      const name = roleNames.get(role.roleId) ?? ""
                      return (
                        <Badge key={role.roleId} variant={role.fixed ? "brand" : "secondary"}>
                          <RoleIcon name={name} aria-hidden />
                          {name}
                          {role.fixed && <Pin aria-label={t("ministries.member.fixed")} />}
                        </Badge>
                      )
                    })}
                  </span>
                }
              />
            </AnimatedItem>
          ))}
        </ListCard>
      )}
      {people.data && <MemberDialog open={open} onOpenChange={setOpen} ministry={ministry} member={selected} people={people.data} />}
    </>
  )
}

function RolesTab({ ministry }: { ministry: MinistryDetail }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<Role | undefined>()
  const [open, setOpen] = useState(false)
  const key = queryKeys.ministry(ministry.id)

  // NOTE: The new order shows at once and animates into place; a failed save puts the old order back.
  const reorder = useMutation({
    mutationFn: (roleIds: number[]) => api.ministries.reorderRoles(ministry.id, roleIds),
    onMutate: async (roleIds) => {
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<MinistryDetail>(key)
      if (previous) {
        const byId = new Map(previous.roles.map((role) => [role.id, role]))
        queryClient.setQueryData<MinistryDetail>(key, { ...previous, roles: roleIds.map((id) => byId.get(id)!) })
      }
      return { previous }
    },
    onError: (error, _ids, context) => {
      if (context?.previous) {
        queryClient.setQueryData(key, context.previous)
      }
      toastError(error)
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: key }),
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
  const holders = (roleId: number) => ministry.members.filter((member) => member.roles.some((role) => role.roleId === roleId)).length

  return (
    <>
      <div className="mb-3 flex items-start justify-between gap-3">
        {ministry.roles.length > 0 ? <p className="text-sm text-pretty text-muted-foreground">{t("ministries.roles.help")}</p> : <span />}
        <Button size="sm" className="shrink-0" onClick={() => openDialog()}>
          <Plus />
          {t("ministries.roles.add")}
        </Button>
      </div>
      {ministry.roles.length === 0 ? (
        <EmptyState icon={ListChecks} title={t("ministries.roles.emptyTitle")}>
          {t("ministries.roles.empty")}
        </EmptyState>
      ) : (
        <ul className="flex flex-col divide-y rounded-2xl border bg-card">
          {ministry.roles.map((role, index) => (
            <motion.li key={role.id} layout="position" transition={spring} className="flex items-center gap-1 pr-2">
              <button
                type="button"
                onClick={() => openDialog(role)}
                className="flex min-w-0 flex-1 items-center gap-3 py-3 pl-4 text-left transition-colors hover:text-brand"
              >
                <span className="tabular w-4 shrink-0 text-sm text-muted-foreground">{index + 1}</span>
                <span className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-primary/12 text-brand">
                  <RoleIcon name={role.name} className="size-4" aria-hidden />
                </span>
                <span className="min-w-0">
                  <span className="block truncate font-medium">{role.name}</span>
                  <span className="block text-xs text-pretty text-muted-foreground">
                    {t("ministries.roles.slots", { count: role.slots })} · {t("ministries.roles.holders", { count: holders(role.id) })}
                  </span>
                </span>
              </button>
              <Button
                variant="ghost"
                size="icon"
                aria-label={t("ministries.roles.moveUp", { name: role.name })}
                disabled={index === 0 || reorder.isPending}
                onClick={() => move(index, -1)}
              >
                <ArrowUp />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                aria-label={t("ministries.roles.moveDown", { name: role.name })}
                disabled={index === ministry.roles.length - 1 || reorder.isPending}
                onClick={() => move(index, 1)}
              >
                <ArrowDown />
              </Button>
            </motion.li>
          ))}
        </ul>
      )}
      <RoleDialog open={open} onOpenChange={setOpen} ministryId={ministry.id} role={selected} />
    </>
  )
}

function ServicesTab({ ministry }: { ministry: MinistryDetail }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const services = useQuery({ queryKey: queryKeys.services, queryFn: api.services.list })
  const key = queryKeys.ministry(ministry.id)
  const save = useMutation({
    mutationFn: (serviceIds: number[]) => api.ministries.setServices(ministry.id, serviceIds),
    onMutate: async (serviceIds) => {
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<MinistryDetail>(key)
      if (previous) {
        queryClient.setQueryData<MinistryDetail>(key, { ...previous, serviceIds })
      }
      return { previous }
    },
    onSuccess: () => toast.success(t("ministries.services.saved"), { id: `ministry-services-${ministry.id}` }),
    onError: (error, _ids, context) => {
      if (context?.previous) {
        queryClient.setQueryData(key, context.previous)
      }
      toastError(error)
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: key }),
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

  if (services.isPending) {
    return <ListSkeleton rows={2} />
  }
  if (services.isError) {
    return <ErrorState error={services.error} onRetry={() => void services.refetch()} retrying={services.isFetching} />
  }
  const active = services.data.filter((service) => service.active || chosen.has(service.id))
  if (active.length === 0) {
    return (
      <EmptyState
        icon={CalendarClock}
        title={t("ministries.services.emptyTitle")}
        action={
          <Button variant="outline" asChild>
            <Link to="/cultos">{t("ministries.services.goToServices")}</Link>
          </Button>
        }
      >
        {t("ministries.services.empty")}
      </EmptyState>
    )
  }

  return (
    <>
      <p className="mb-3 text-sm text-pretty text-muted-foreground">{t("ministries.services.help")}</p>
      <ul className="flex flex-col divide-y rounded-2xl border bg-card">
        {active.map((service) => {
          const checkboxId = `service-${service.id}`
          const communion = communionLabel(service)
          return (
            <li key={service.id}>
              <label
                htmlFor={checkboxId}
                className={cn("flex min-h-14 cursor-pointer items-center gap-3 px-4 py-3 transition-colors hover:bg-accent/40", !service.active && "opacity-60")}
              >
                <Checkbox id={checkboxId} checked={chosen.has(service.id)} onCheckedChange={(checked) => toggle(service.id, checked === true)} />
                <span className="min-w-0 flex-1">
                  <span className="block font-medium text-pretty">{service.name}</span>
                  <span className="block text-sm text-muted-foreground">
                    {serviceLabel(service)}
                    {communion && ` · ${communion}`}
                  </span>
                </span>
              </label>
            </li>
          )
        })}
      </ul>
    </>
  )
}

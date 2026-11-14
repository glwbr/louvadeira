import { revalidateLogic } from "@tanstack/react-form"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Pin } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"
import { toast } from "sonner"

import { ConfirmButton } from "@/components/confirm-button"
import { FieldShell, useAppForm } from "@/components/form"
import { RoleIcon } from "@/components/role-icon"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Switch } from "@/components/ui/switch"
import { api, queryKeys, type Member, type MemberInput, type MemberRole, type MinistryDetail, type Person } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { cn } from "@/lib/utils"

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
      <DialogContent autoFocusFirst={!member}>
        <MemberForm key={member?.personId ?? "new"} ministry={ministry} member={member} people={people} onDone={() => onOpenChange(false)} />
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
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const memberIds = new Set(ministry.members.map((m) => m.personId))
  const candidates = people.filter((p) => !memberIds.has(p.id))

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.ministry(ministry.id) })
    queryClient.invalidateQueries({ queryKey: queryKeys.ministries })
  }
  const save = useMutation({
    mutationFn: ({ personId, input }: { personId: number; input: MemberInput }) => api.ministries.putMember(ministry.id, personId, input),
    onSuccess: () => {
      refresh()
      toast.success(member ? t("ministries.member.updated") : t("ministries.member.added"))
      onDone()
    },
    onError: toastError,
  })
  const remove = useMutation({
    mutationFn: () => api.ministries.removeMember(ministry.id, member!.personId),
    onSuccess: () => {
      refresh()
      toast.success(t("ministries.member.removed"))
      onDone()
    },
    onError: toastError,
  })

  const form = useAppForm({
    defaultValues: {
      personId: member ? String(member.personId) : "",
      isLeader: member?.isLeader ?? false,
      roles: member?.roles ?? ([] as MemberRole[]),
    },
    validationLogic: revalidateLogic(),
    validators: {
      onDynamic: ({ value }) => (value.personId ? undefined : { fields: { personId: t("ministries.member.personRequired") } }),
    },
    onSubmit: async ({ value }) => {
      await save
        .mutateAsync({ personId: Number(value.personId), input: { isLeader: value.isLeader, roles: value.roles } })
        .catch(() => undefined)
    },
  })

  if (!member && candidates.length === 0) {
    return (
      <div className="flex flex-col gap-5">
        <DialogHeader>
          <DialogTitle>{t("ministries.member.allInTitle")}</DialogTitle>
          <DialogDescription>{t("ministries.member.allIn")}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button asChild>
            <Link to="/pessoas">{t("ministries.member.goToPeople")}</Link>
          </Button>
        </DialogFooter>
      </div>
    )
  }

  return (
    <form.AppForm>
      <form.Form>
        <DialogHeader>
          <DialogTitle>{member ? member.name : t("ministries.member.addTitle")}</DialogTitle>
          <DialogDescription>{t("ministries.member.lead")}</DialogDescription>
        </DialogHeader>

        {!member && (
          <form.AppField name="personId">
            {(field) => (
              <field.SelectField
                label={t("ministries.member.person")}
                placeholder={t("ministries.member.personPlaceholder")}
                options={candidates.map((person) => ({ value: String(person.id), label: person.name }))}
              />
            )}
          </form.AppField>
        )}

        <form.Field name="roles">
          {(field) => {
            const selected = new Map(field.state.value.map((role) => [role.roleId, role]))
            const toggle = (roleId: number, on: boolean) =>
              field.handleChange(
                on ? [...field.state.value, { roleId, fixed: false }] : field.state.value.filter((role) => role.roleId !== roleId)
              )
            const setFixed = (roleId: number, fixed: boolean) =>
              field.handleChange(field.state.value.map((role) => (role.roleId === roleId ? { ...role, fixed } : role)))
            return (
              <FieldShell label={t("ministries.member.roles")}>
                <ul aria-label={t("ministries.member.roles")} className="flex flex-col divide-y overflow-hidden rounded-xl border">
                  {ministry.roles.map((role) => {
                    const current = selected.get(role.id)
                    const checkboxId = `role-${role.id}`
                    return (
                      <li key={role.id} className={cn("flex min-h-12 items-center gap-1 pr-2 transition-colors", current && "bg-primary/6")}>
                        <label htmlFor={checkboxId} className="flex min-h-12 min-w-0 flex-1 cursor-pointer items-center gap-3 px-3">
                          <Checkbox id={checkboxId} checked={Boolean(current)} onCheckedChange={(checked) => toggle(role.id, checked === true)} />
                          <RoleIcon name={role.name} className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                          <span className="truncate text-sm font-medium">{role.name}</span>
                        </label>
                        {current && (
                          <label className="flex h-10 cursor-pointer items-center gap-2 rounded-lg px-2 text-xs text-muted-foreground hover:bg-accent/50">
                            <Pin className="size-3.5" aria-hidden />
                            {t("ministries.member.fixed")}
                            <Switch checked={current.fixed} onCheckedChange={(fixed) => setFixed(role.id, fixed)} />
                          </label>
                        )}
                      </li>
                    )
                  })}
                </ul>
              </FieldShell>
            )
          }}
        </form.Field>

        <form.AppField name="isLeader">
          {(field) => <field.SwitchField label={t("ministries.member.leader")} description={t("ministries.member.leaderHint")} />}
        </form.AppField>

        <DialogFooter className="sm:justify-between">
          {member ? (
            <ConfirmButton
              label={t("ministries.member.remove")}
              title={t("ministries.member.removeTitle", { name: member.name })}
              description={t("ministries.member.removeLead")}
              onConfirm={() => remove.mutate()}
              pending={remove.isPending}
            />
          ) : (
            <span className="hidden sm:block" />
          )}
          <form.SubmitButton pendingLabel={t("common.saving")}>{t("common.save")}</form.SubmitButton>
        </DialogFooter>
      </form.Form>
    </form.AppForm>
  )
}

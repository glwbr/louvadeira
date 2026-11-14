import { revalidateLogic } from "@tanstack/react-form"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { ConfirmButton } from "@/components/confirm-button"
import { useAppForm } from "@/components/form"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api, queryKeys, type Role, type RoleInput } from "@/lib/api"
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
      <DialogContent autoFocusFirst={!role}>{open && <RoleForm ministryId={ministryId} role={role} onDone={() => onOpenChange(false)} />}</DialogContent>
    </Dialog>
  )
}

const MAX_SLOTS = 20

function RoleForm({ ministryId, role, onDone }: { ministryId: number; role?: Role; onDone: () => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.ministry(ministryId) })

  const save = useMutation({
    mutationFn: async (input: RoleInput) => {
      if (role) {
        await api.ministries.updateRole(ministryId, role.id, input)
      } else {
        await api.ministries.createRole(ministryId, input)
      }
    },
    onSuccess: () => {
      refresh()
      toast.success(role ? t("ministries.role.updated") : t("ministries.role.created"))
      onDone()
    },
    onError: toastError,
  })
  const remove = useMutation({
    mutationFn: () => api.ministries.deleteRole(ministryId, role!.id),
    onSuccess: () => {
      refresh()
      toast.success(t("ministries.role.removed"))
      onDone()
    },
    onError: toastError,
  })

  const form = useAppForm({
    defaultValues: { name: role?.name ?? "", slots: String(role?.slots ?? 1) },
    validationLogic: revalidateLogic(),
    validators: {
      onDynamic: ({ value }) => {
        const slots = Number(value.slots)
        const fields: { name?: string; slots?: string } = {}
        if (!value.name.trim()) {
          fields.name = t("ministries.role.nameRequired")
        }
        if (!Number.isInteger(slots) || slots < 1 || slots > MAX_SLOTS) {
          fields.slots = t("ministries.role.slotsInvalid")
        }
        return Object.keys(fields).length > 0 ? { fields } : undefined
      },
    },
    onSubmit: async ({ value }) => {
      await save.mutateAsync({ name: value.name.trim(), slots: Number(value.slots) }).catch(() => undefined)
    },
  })

  return (
    <form.AppForm>
      <form.Form>
        <DialogHeader>
          <DialogTitle>{role ? t("ministries.role.editTitle") : t("ministries.role.newTitle")}</DialogTitle>
          <DialogDescription>{t("ministries.role.lead")}</DialogDescription>
        </DialogHeader>
        <form.AppField name="name">
          {(field) => <field.TextField label={t("ministries.role.name")} autoFocus={!role} autoComplete="off" />}
        </form.AppField>
        <form.AppField name="slots">
          {(field) => (
            <field.TextField
              label={t("ministries.role.slots")}
              hint={t("ministries.role.slotsHint")}
              type="number"
              inputMode="numeric"
              min={1}
              max={MAX_SLOTS}
            />
          )}
        </form.AppField>
        <DialogFooter className="sm:justify-between">
          {role ? (
            <ConfirmButton
              label={t("ministries.role.remove")}
              title={t("ministries.role.removeTitle", { name: role.name })}
              description={t("ministries.role.removeLead")}
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

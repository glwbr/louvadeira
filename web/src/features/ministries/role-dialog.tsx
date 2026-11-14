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
      <DialogContent autoFocusFirst={!role}><RoleForm key={role?.id ?? "new"} ministryId={ministryId} role={role} onDone={() => onOpenChange(false)} /></DialogContent>
    </Dialog>
  )
}

const MAX_SLOTS = 20
const EVERY_SLOT = "all"


/**
 * NOTE: Every count below the role's slots, largest first, between "every slot" and "nobody".
 */
function fewerThan(slots: number): number[] {
  const below = Number.isInteger(slots) && slots > 1 ? Math.min(slots, MAX_SLOTS) - 1 : 0
  return Array.from({ length: below }, (_, i) => below - i)
}

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
    defaultValues: {
      name: role?.name ?? "",
      slots: String(role?.slots ?? 1),
      minimum: role?.minimum == null ? EVERY_SLOT : String(role.minimum),
    },
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
      const minimum = value.minimum === EVERY_SLOT ? null : Number(value.minimum)
      await save.mutateAsync({ name: value.name.trim(), slots: Number(value.slots), minimum }).catch(() => undefined)
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
        <form.AppField
          name="slots"
          listeners={{
            onChange: ({ value }) => {
              const minimum = form.getFieldValue("minimum")
              if (minimum !== EVERY_SLOT && Number(minimum) >= Number(value)) {
                form.setFieldValue("minimum", EVERY_SLOT)
              }
            },
          }}
        >
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
        <form.Subscribe selector={(state) => Number(state.values.slots)}>
          {(slots) => (
            <form.AppField name="minimum">
              {(field) => (
                <field.SelectField
                  label={t("ministries.role.minimum")}
                  hint={t("ministries.role.minimumHint")}
                  options={[
                    { value: EVERY_SLOT, label: t("ministries.role.minimumAll") },
                    ...fewerThan(slots).map((count) => ({
                      value: String(count),
                      label: t("ministries.role.minimumCount", { count }),
                    })),
                    { value: "0", label: t("ministries.role.minimumNone") },
                  ]}
                />
              )}
            </form.AppField>
          )}
        </form.Subscribe>
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

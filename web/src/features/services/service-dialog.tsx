import { revalidateLogic } from "@tanstack/react-form"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { useAppForm } from "@/components/form"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api, queryKeys, type Service, type ServiceInput } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { ordinalOfMonth, WEEKDAY_INDEXES, weekdayName } from "@/lib/format"

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  service?: Service
}

export function ServiceDialog({ open, onOpenChange, service }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent autoFocusFirst={!service}>
        {open && <ServiceForm service={service} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

const NO_COMMUNION = "none"

function ServiceForm({ service, onDone }: { service?: Service; onDone: () => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const save = useMutation({
    mutationFn: (input: ServiceInput) => (service ? api.services.update(service.id, input) : api.services.create(input)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.services })
      toast.success(service ? t("services.dialog.updated") : t("services.dialog.created"))
      onDone()
    },
    onError: toastError,
  })

  const form = useAppForm({
    defaultValues: {
      name: service?.name ?? "",
      weekday: String(service?.weekday ?? 0),
      startTime: service?.startTime ?? "19:00",
      active: service?.active ?? true,
      communionWeek: service?.communionWeek ? String(service.communionWeek) : NO_COMMUNION,
    },
    validationLogic: revalidateLogic(),
    validators: {
      onDynamic: ({ value }) => {
        const fields: { name?: string; startTime?: string } = {}
        if (!value.name.trim()) {
          fields.name = t("services.dialog.nameRequired")
        }
        if (!value.startTime) {
          fields.startTime = t("services.dialog.timeRequired")
        }
        return Object.keys(fields).length > 0 ? { fields } : undefined
      },
    },
    onSubmit: async ({ value }) => {
      await save
        .mutateAsync({
          name: value.name.trim(),
          weekday: Number(value.weekday),
          startTime: value.startTime,
          active: value.active,
          communionWeek: value.communionWeek === NO_COMMUNION ? null : Number(value.communionWeek),
        })
        .catch(() => undefined)
    },
  })

  return (
    <form.AppForm>
      <form.Form>
        <DialogHeader>
          <DialogTitle>{service ? t("services.dialog.editTitle") : t("services.dialog.newTitle")}</DialogTitle>
          <DialogDescription>{t("services.dialog.lead")}</DialogDescription>
        </DialogHeader>
        <form.AppField name="name">
          {(field) => (
            <field.TextField
              label={t("services.dialog.name")}
              placeholder={t("services.dialog.namePlaceholder")}
              autoFocus={!service}
              autoComplete="off"
            />
          )}
        </form.AppField>
        <div className="grid grid-cols-2 gap-3">
          <form.AppField name="weekday">
            {(field) => (
              <field.SelectField
                label={t("services.dialog.weekday")}
                options={WEEKDAY_INDEXES.map((index) => ({ value: String(index), label: weekdayName(index) }))}
              />
            )}
          </form.AppField>
          <form.AppField name="startTime">
            {(field) => <field.TextField type="time" label={t("services.dialog.time")} />}
          </form.AppField>
        </div>
        <form.Subscribe selector={(state) => Number(state.values.weekday)}>
          {(weekday) => (
            <form.AppField name="communionWeek">
              {(field) => (
                <field.SelectField
                  label={t("services.dialog.communion")}
                  options={[
                    { value: NO_COMMUNION, label: t("services.dialog.noCommunion") },
                    ...[1, 2, 3, 4].map((week) => ({ value: String(week), label: ordinalOfMonth(week, weekday) })),
                  ]}
                />
              )}
            </form.AppField>
          )}
        </form.Subscribe>
        {service && (
          <form.AppField name="active">
            {(field) => <field.SwitchField label={t("services.dialog.active")} description={t("services.dialog.activeHint")} />}
          </form.AppField>
        )}
        <DialogFooter>
          <form.SubmitButton pendingLabel={t("common.saving")}>{t("common.save")}</form.SubmitButton>
        </DialogFooter>
      </form.Form>
    </form.AppForm>
  )
}

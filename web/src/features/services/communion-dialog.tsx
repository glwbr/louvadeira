import { revalidateLogic } from "@tanstack/react-form"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Loader2 } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { useAppForm } from "@/components/form"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api, queryKeys, type Service } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { ordinalOfMonth, serviceLabel } from "@/lib/format"

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  services: Service[]
  current?: Service
}

export function CommunionDialog({ open, onOpenChange, services, current }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <CommunionForm services={services} current={current} onDone={() => onOpenChange(false)} />
      </DialogContent>
    </Dialog>
  )
}

const WEEKS = [1, 2, 3, 4]

function CommunionForm({ services, current, onDone }: { services: Service[]; current?: Service; onDone: () => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const save = useMutation({
    mutationFn: ({ serviceId, week }: { serviceId: number | null; week: number | null }) => api.services.setCommunion(serviceId, week),
    onSuccess: (_, { serviceId }) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.services })
      queryClient.invalidateQueries({ queryKey: ["schedules"] })
      toast.success(serviceId === null ? t("services.communion.cleared") : t("services.communion.saved"))
      onDone()
    },
    onError: toastError,
  })

  const form = useAppForm({
    defaultValues: {
      serviceId: current ? String(current.id) : "",
      week: current?.communionWeek ? String(current.communionWeek) : "",
    },
    validationLogic: revalidateLogic(),
    validators: {
      onDynamic: ({ value }) => {
        const fields: { serviceId?: string; week?: string } = {}
        if (!value.serviceId) {
          fields.serviceId = t("services.communion.serviceRequired")
        }
        if (!value.week) {
          fields.week = t("services.communion.weekRequired")
        }
        return Object.keys(fields).length > 0 ? { fields } : undefined
      },
    },
    onSubmit: async ({ value }) => {
      await save.mutateAsync({ serviceId: Number(value.serviceId), week: Number(value.week) }).catch(() => undefined)
    },
  })

  const byId = new Map(services.map((service) => [String(service.id), service]))

  return (
    <form.AppForm>
      <form.Form>
        <DialogHeader>
          <DialogTitle>{t("services.communion.dialogTitle")}</DialogTitle>
          <DialogDescription>{t("services.communion.dialogLead")}</DialogDescription>
        </DialogHeader>
        <form.AppField name="serviceId">
          {(field) => (
            <field.SelectField
              label={t("services.communion.service")}
              placeholder={t("services.communion.servicePlaceholder")}
              options={services.map((service) => ({ value: String(service.id), label: `${service.name} · ${serviceLabel(service)}` }))}
            />
          )}
        </form.AppField>
        <form.Subscribe selector={(state) => byId.get(state.values.serviceId)}>
          {(service) => (
            <form.AppField name="week">
              {(field) => (
                <field.SelectField
                  label={t("services.communion.week")}
                  placeholder={t("services.communion.weekPlaceholder")}
                  disabled={!service}
                  options={WEEKS.map((week) => ({
                    value: String(week),
                    label: service ? ordinalOfMonth(week, service.weekday) : String(week),
                  }))}
                />
              )}
            </form.AppField>
          )}
        </form.Subscribe>
        <DialogFooter className="sm:justify-between">
          {current ? (
            <Button
              type="button"
              variant="ghost"
              className="text-danger hover:text-danger"
              disabled={save.isPending}
              onClick={() => save.mutate({ serviceId: null, week: null })}
            >
              {save.isPending && save.variables?.serviceId === null && <Loader2 className="animate-spin" />}
              {t("services.communion.clear")}
            </Button>
          ) : (
            <span className="hidden sm:block" />
          )}
          <form.SubmitButton pendingLabel={t("common.saving")}>{t("common.save")}</form.SubmitButton>
        </DialogFooter>
      </form.Form>
    </form.AppForm>
  )
}

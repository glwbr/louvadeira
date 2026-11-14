import { revalidateLogic } from "@tanstack/react-form"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { ConfirmButton } from "@/components/confirm-button"
import { useAppForm } from "@/components/form"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api, queryKeys, type Ministry, type MinistryInput, type RotationUnit } from "@/lib/api"
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
      <DialogContent autoFocusFirst={!ministry}>{open && <MinistryForm ministry={ministry} onDone={() => onOpenChange(false)} />}</DialogContent>
    </Dialog>
  )
}

const NO_GROUP = "none"

function MinistryForm({ ministry, onDone }: { ministry?: Ministry; onDone: () => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const groups = useQuery({ queryKey: queryKeys.whatsappGroups, queryFn: api.whatsapp.groups, retry: false, staleTime: 5 * 60_000 })

  const save = useMutation({
    mutationFn: async (input: MinistryInput) => {
      if (ministry) {
        await api.ministries.update(ministry.id, input)
        return ministry.id
      }
      return (await api.ministries.create(input)).id
    },
    onSuccess: (id) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.ministries })
      toast.success(ministry ? t("ministries.dialog.updated") : t("ministries.dialog.created"))
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
      toast.success(t("ministries.dialog.archived"))
      onDone()
      navigate("/ministerios")
    },
    onError: toastError,
  })

  const form = useAppForm({
    defaultValues: {
      name: ministry?.name ?? "",
      rotationUnit: (ministry?.rotationUnit ?? "week") as RotationUnit,
      whatsappGroup: ministry?.whatsappGroup || NO_GROUP,
    },
    validationLogic: revalidateLogic(),
    validators: {
      onDynamic: ({ value }) => (value.name.trim() ? undefined : { fields: { name: t("ministries.dialog.nameRequired") } }),
    },
    onSubmit: async ({ value }) => {
      const input = { ...value, name: value.name.trim(), whatsappGroup: value.whatsappGroup === NO_GROUP ? "" : value.whatsappGroup }
      await save.mutateAsync(input).catch(() => undefined)
    },
  })

  const groupOptions = [
    { value: NO_GROUP, label: t("ministries.dialog.noGroup") },
    ...(groups.data ?? []).map((group) => ({ value: group.id, label: group.name })),
  ]
  // NOTE: Keep the saved group selectable while the list is unavailable, so its name does not vanish.
  if (ministry?.whatsappGroup && !groupOptions.some((option) => option.value === ministry.whatsappGroup)) {
    groupOptions.push({ value: ministry.whatsappGroup, label: t("ministries.dialog.savedGroup") })
  }

  return (
    <form.AppForm>
      <form.Form>
        <DialogHeader>
          <DialogTitle>{ministry ? t("ministries.dialog.editTitle") : t("ministries.dialog.newTitle")}</DialogTitle>
          <DialogDescription>{t("ministries.dialog.lead")}</DialogDescription>
        </DialogHeader>
        <form.AppField name="name">
          {(field) => <field.TextField label={t("ministries.dialog.name")} autoFocus={!ministry} autoComplete="off" />}
        </form.AppField>
        <form.AppField name="rotationUnit">
          {(field) => (
            <field.SelectField
              label={t("ministries.dialog.rotation")}
              hint={t("ministries.dialog.rotationHint")}
              options={(["week", "service"] as const).map((unit) => ({ value: unit, label: rotationLabel(unit) }))}
            />
          )}
        </form.AppField>
        <form.AppField name="whatsappGroup">
          {(field) => (
            <field.SelectField
              label={t("ministries.dialog.group")}
              disabled={!groups.data}
              placeholder={t("ministries.dialog.groupLoading")}
              hint={groups.isError ? t("ministries.dialog.groupOffline") : t("ministries.dialog.groupHint")}
              options={groupOptions}
            />
          )}
        </form.AppField>
        <DialogFooter className="sm:justify-between">
          {ministry ? (
            <ConfirmButton
              label={t("ministries.dialog.archive")}
              title={t("ministries.dialog.archiveTitle", { name: ministry.name })}
              description={t("ministries.dialog.archiveLead")}
              onConfirm={() => archive.mutate()}
              pending={archive.isPending}
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

import { revalidateLogic } from "@tanstack/react-form"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Loader2, MessageCircle } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { ConfirmButton } from "@/components/confirm-button"
import { useAppForm } from "@/components/form"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api, queryKeys, type Person, type PersonInput } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { firstName, formatPhone, maskPhone, phoneDigits } from "@/lib/format"

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  person?: Person
  canArchive?: boolean
}

export function PersonDialog({ open, onOpenChange, person, canArchive }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent autoFocusFirst={!person}>
        {open && <PersonForm person={person} canArchive={canArchive} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function PersonForm({ person, canArchive, onDone }: { person?: Person; canArchive?: boolean; onDone: () => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const health = useQuery({ queryKey: queryKeys.health, queryFn: api.health, meta: { background: true } })
  const savedPhone = person ? formatPhone(person.phone) : ""

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.people })
    queryClient.invalidateQueries({ queryKey: queryKeys.ministries })
  }
  const save = useMutation({
    mutationFn: (input: PersonInput) => (person ? api.people.update(person.id, input) : api.people.create(input)),
    onSuccess: () => {
      refresh()
      toast.success(person ? t("people.dialog.updated") : t("people.dialog.created"))
      onDone()
    },
    onError: toastError,
  })
  const sendLink = useMutation({
    mutationFn: () => api.people.sendLink(person!.id),
    onSuccess: () => toast.success(t("people.dialog.linkSent", { name: firstName(person!.name) })),
    onError: toastError,
  })
  const archive = useMutation({
    mutationFn: () => api.people.archive(person!.id),
    onSuccess: () => {
      refresh()
      toast.success(t("people.dialog.archived"))
      onDone()
    },
    onError: toastError,
  })

  const form = useAppForm({
    defaultValues: { name: person?.name ?? "", phone: savedPhone },
    validationLogic: revalidateLogic(),
    validators: {
      onDynamic: ({ value }) => {
        const fields: { name?: string; phone?: string } = {}
        if (!value.name.trim()) {
          fields.name = t("people.dialog.nameRequired")
        }
        const digits = phoneDigits(value.phone).length
        if (value.phone && digits !== 10 && digits !== 11) {
          fields.phone = t("people.dialog.phoneInvalid")
        }
        return Object.keys(fields).length > 0 ? { fields } : undefined
      },
    },
    onSubmit: async ({ value }) => {
      await save.mutateAsync({ name: value.name.trim(), phone: value.phone }).catch(() => undefined)
    },
  })

  const whatsappOff = health.data?.whatsapp.mode === "off"

  return (
    <form.AppForm>
      <form.Form>
        <DialogHeader>
          <DialogTitle>{person ? t("people.dialog.editTitle") : t("people.dialog.newTitle")}</DialogTitle>
          <DialogDescription>{t("people.dialog.lead")}</DialogDescription>
        </DialogHeader>
        <form.AppField name="name">
          {(field) => <field.TextField label={t("people.dialog.name")} autoFocus={!person} autoComplete="off" />}
        </form.AppField>
        <form.AppField name="phone">
          {(field) => (
            <field.TextField
              label={t("people.dialog.phone")}
              type="tel"
              inputMode="tel"
              autoComplete="off"
              placeholder={t("people.dialog.phonePlaceholder")}
              format={maskPhone}
            />
          )}
        </form.AppField>
        {person?.phone && (
          <form.Subscribe selector={(state) => state.values.phone !== savedPhone}>
            {(phoneChanged) => (
              <div className="flex flex-col gap-1.5">
                <Button
                  type="button"
                  variant="outline"
                  disabled={sendLink.isPending || phoneChanged || whatsappOff}
                  onClick={() => sendLink.mutate()}
                >
                  {sendLink.isPending ? <Loader2 className="animate-spin" /> : <MessageCircle />}
                  {sendLink.isPending ? t("people.dialog.sendingLink") : t("people.dialog.sendLink")}
                </Button>
                {(phoneChanged || whatsappOff) && (
                  <p className="text-xs text-pretty text-muted-foreground">
                    {phoneChanged ? t("people.dialog.saveFirst") : t("people.dialog.whatsappOff")}
                  </p>
                )}
              </div>
            )}
          </form.Subscribe>
        )}
        <DialogFooter className="sm:justify-between">
          {person && canArchive ? (
            <ConfirmButton
              label={t("people.dialog.archive")}
              title={t("people.dialog.archiveTitle", { name: person.name })}
              description={t("people.dialog.archiveLead")}
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

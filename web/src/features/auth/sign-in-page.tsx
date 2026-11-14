import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { CloudOff, Loader2, MessageCircleHeart, RotateCw } from "lucide-react"
import { AnimatePresence, motion } from "motion/react"
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Navigate, useSearchParams } from "react-router"

import { Wordmark } from "@/components/brand"
import { useAppForm } from "@/components/form"
import { quick } from "@/components/motion"
import { Button } from "@/components/ui/button"
import { ApiError, api, queryKeys } from "@/lib/api"
import { maskPhone, phoneDigits } from "@/lib/format"

export function SignInPage() {
  const { t } = useTranslation()
  const [params] = useSearchParams()
  const queryClient = useQueryClient()
  const token = params.get("token")

  /**
   * NOTE: Redeeming is a query keyed by the token, so React running the page twice in development, or the page
   * rendering again, still spends the single-use link only once.
   */
  const redeem = useQuery({
    queryKey: ["redeem", token],
    enabled: Boolean(token),
    retry: false,
    staleTime: Infinity,
    gcTime: Infinity,
    queryFn: async () => {
      const person = await api.redeem(token!)
      queryClient.setQueryData(queryKeys.me, person)
      return person
    },
  })

  if (redeem.data) {
    return <Navigate to={redeem.data.isAdmin ? "/" : "/minha-escala"} replace />
  }

  const expired = redeem.error instanceof ApiError && redeem.error.status === 401

  if (token && !redeem.isError) {
    return (
      <Centered>
        <Loader2 className="size-7 animate-spin text-brand" aria-hidden />
        <h1 className="font-display text-2xl font-semibold" role="status">
          {t("signIn.redeeming")}
        </h1>
      </Centered>
    )
  }

  if (token && redeem.isError && !expired) {
    return (
      <Centered>
        <span className="flex size-12 items-center justify-center rounded-2xl bg-danger/12 text-danger">
          <CloudOff className="size-6" />
        </span>
        <h1 className="font-display text-[1.75rem] leading-tight font-semibold text-balance">{t("signIn.failedTitle")}</h1>
        <p className="text-sm text-pretty text-muted-foreground">{redeem.error.message}</p>
        <Button size="lg" className="w-full" onClick={() => void redeem.refetch()} disabled={redeem.isFetching}>
          {redeem.isFetching ? <Loader2 className="animate-spin" /> : <RotateCw />}
          {t("common.tryAgain")}
        </Button>
      </Centered>
    )
  }

  return (
    <Centered>
      <h1 className="font-display text-[1.75rem] leading-tight font-semibold text-balance">
        {expired ? t("signIn.expiredTitle") : t("signIn.title")}
      </h1>
      <p className="text-sm text-pretty text-muted-foreground">{expired ? t("signIn.expiredLead") : t("signIn.lead")}</p>
      <PhoneForm />
    </Centered>
  )
}

function PhoneForm() {
  const { t } = useTranslation()
  const request = useMutation({ mutationFn: (phone: string) => api.requestLink(phone) })
  const form = useAppForm({
    defaultValues: { phone: "" },
    onSubmit: async ({ value }) => {
      await request.mutateAsync(phoneDigits(value.phone)).catch(() => undefined)
    },
  })

  const validPhone = ({ value }: { value: string }) => {
    const digits = phoneDigits(value).length
    return digits === 10 || digits === 11 ? undefined : t("signIn.phoneInvalid")
  }

  return (
    <div className="w-full text-left">
      <AnimatePresence mode="wait" initial={false}>
        {request.isSuccess ? (
          <motion.div
            key="sent"
            initial={{ opacity: 0, y: 8, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -4 }}
            transition={quick}
            className="flex flex-col items-center gap-3 rounded-2xl border bg-card px-5 py-6 text-center"
            role="status"
          >
            <span className="flex size-12 items-center justify-center rounded-2xl bg-success/12 text-success">
              <MessageCircleHeart className="size-6" />
            </span>
            <p className="font-medium">{t("signIn.sentTitle")}</p>
            <p className="text-sm text-pretty text-muted-foreground">{t("signIn.sent")}</p>
            <Button
              variant="ghost"
              size="sm"
              className="mt-1 text-muted-foreground"
              onClick={() => {
                request.reset()
                form.reset()
              }}
            >
              {t("signIn.otherNumber")}
            </Button>
          </motion.div>
        ) : (
          <motion.div key="form" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={quick}>
            <form.AppForm>
              <form.Form>
                <form.AppField name="phone" validators={{ onBlur: validPhone, onSubmit: validPhone }}>
                  {(field) => (
                    <field.TextField
                      label={t("signIn.phoneLabel")}
                      type="tel"
                      inputMode="tel"
                      autoComplete="tel-national"
                      placeholder={t("signIn.phonePlaceholder")}
                      format={maskPhone}
                    />
                  )}
                </form.AppField>
                {request.isError && (
                  <p role="alert" className="-mt-2 text-sm text-danger">
                    {request.error.message}
                  </p>
                )}
                <form.SubmitButton pendingLabel={t("signIn.sending")} className="h-12 w-full text-base">
                  {t("signIn.submit")}
                </form.SubmitButton>
              </form.Form>
            </form.AppForm>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

function Centered({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-svh flex-col px-6 pt-[max(1.5rem,env(safe-area-inset-top))] pb-[max(1.5rem,env(safe-area-inset-bottom))]">
      <Wordmark className="mx-auto" />
      <main className="mx-auto flex w-full max-w-sm flex-1 flex-col items-center justify-center gap-4 pb-16 text-center">
        {children}
      </main>
    </div>
  )
}

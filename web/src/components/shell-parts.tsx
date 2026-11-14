import { useIsFetching, useIsMutating, useQueryClient } from "@tanstack/react-query"
import { LogOut } from "lucide-react"
import { AnimatePresence, motion } from "motion/react"
import { useTranslation } from "react-i18next"

import { ConfirmButton } from "@/components/confirm-button"
import { api } from "@/lib/api"

/**
 * NOTE: A thin bar under the top edge while something is loading or saving. Polling queries mark themselves with
 * meta.background so a refresh every few seconds does not blink it; it also waits 200ms to skip instant answers.
 */
export function ActivityBar() {
  const fetching = useIsFetching({ predicate: (query) => !query.meta?.background })
  const mutating = useIsMutating()
  return (
    <AnimatePresence>
      {fetching + mutating > 0 && (
        <motion.div
          aria-hidden
          className="pointer-events-none absolute inset-x-0 top-0 h-0.5 overflow-hidden"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1, transition: { delay: 0.2, duration: 0.2 } }}
          exit={{ opacity: 0, transition: { duration: 0.3 } }}
        >
          <div className="activity-sweep h-full w-1/3 rounded-full bg-brand" />
        </motion.div>
      )}
    </AnimatePresence>
  )
}

export function SignOutButton({ to }: { to: string }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const logout = async () => {
    await api.logout().catch(() => undefined)
    queryClient.clear()
    window.location.assign(to)
  }
  return (
    <ConfirmButton
      tone="outline"
      size="sm"
      className="border-transparent bg-transparent text-muted-foreground shadow-none hover:text-foreground dark:border-transparent dark:bg-transparent"
      icon={<LogOut />}
      label={t("common.signOut")}
      title={t("common.signOutTitle")}
      description={t("common.signOutLead")}
      onConfirm={() => void logout()}
    />
  )
}

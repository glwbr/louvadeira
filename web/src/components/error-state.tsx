import { CloudOff, Loader2, RotateCw } from "lucide-react"
import { useTranslation } from "react-i18next"

import { EmptyState } from "@/components/empty-state"
import { Button } from "@/components/ui/button"

type Props = {
  error: Error | null
  onRetry: () => void
  retrying?: boolean
  title?: string
  className?: string
}

/**
 * NOTE: A failed request never shows the empty state: "you are not on any schedule" read on a bad connection would be
 * a lie someone acts on.
 */
export function ErrorState({ error, onRetry, retrying = false, title, className }: Props) {
  const { t } = useTranslation()
  return (
    <EmptyState
      icon={CloudOff}
      tone="danger"
      title={title ?? t("common.loadErrorTitle")}
      className={className}
      action={
        <Button variant="outline" onClick={onRetry} disabled={retrying}>
          {retrying ? <Loader2 className="animate-spin" /> : <RotateCw />}
          {t("common.tryAgain")}
        </Button>
      }
    >
      {error?.message}
    </EmptyState>
  )
}

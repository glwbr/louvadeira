import { toast } from "sonner"

import { t } from "@/lib/i18n"

export function toastError(error: unknown) {
  const message = error instanceof Error && error.message ? error.message : t("common.genericError")
  toast.error(message.charAt(0).toUpperCase() + message.slice(1))
}

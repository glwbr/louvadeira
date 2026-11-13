import { toast } from "sonner"

export function toastError(error: unknown) {
  const message = error instanceof Error ? error.message : "Algo deu errado. Tente de novo."
  toast.error(message.charAt(0).toUpperCase() + message.slice(1))
}

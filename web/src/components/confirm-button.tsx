import { Loader2 } from "lucide-react"
import type { ComponentProps, ReactNode } from "react"
import { useTranslation } from "react-i18next"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

type Props = {
  label: ReactNode
  title: ReactNode
  description: ReactNode
  onConfirm: () => void
  confirmLabel?: ReactNode
  pending?: boolean
  tone?: "destructive" | "primary" | "outline"
  size?: ComponentProps<typeof Button>["size"]
  icon?: ReactNode
  className?: string
  "aria-label"?: string
  children?: ReactNode
}

/**
 * NOTE: A button that asks before acting. children, when given, go between the description and the buttons, for
 * example the list of what is about to change.
 */
export function ConfirmButton({
  label,
  title,
  description,
  onConfirm,
  confirmLabel,
  pending,
  tone = "destructive",
  size,
  icon,
  className,
  children,
  ...rest
}: Props) {
  const { t } = useTranslation()
  const variant = tone === "destructive" ? "ghost" : tone === "primary" ? "default" : "outline"

  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>
        <Button
          type="button"
          variant={variant}
          size={size}
          disabled={pending}
          aria-label={rest["aria-label"]}
          className={cn(tone === "destructive" && "text-danger hover:text-danger", className)}
        >
          {pending ? <Loader2 className="animate-spin" /> : icon}
          {label}
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        {children}
        <AlertDialogFooter>
          <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
          <AlertDialogAction variant={tone === "destructive" ? "destructive" : "default"} onClick={onConfirm}>
            {confirmLabel ?? label}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

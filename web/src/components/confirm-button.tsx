import type { ReactNode } from "react"

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

type Props = {
  label: string
  title: string
  description: string
  onConfirm: () => void
  pending?: boolean
  tone?: "destructive" | "primary" | "outline"
  icon?: ReactNode
}

export function ConfirmButton({ label, title, description, onConfirm, pending, tone = "destructive", icon }: Props) {
  const trigger =
    tone === "destructive" ? (
      <Button type="button" variant="ghost" className="text-destructive hover:text-destructive" disabled={pending}>
        {icon}
        {label}
      </Button>
    ) : (
      <Button type="button" variant={tone === "primary" ? "default" : "outline"} disabled={pending}>
        {icon}
        {label}
      </Button>
    )

  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>{trigger}</AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancelar</AlertDialogCancel>
          <AlertDialogAction variant={tone === "destructive" ? "destructive" : "default"} onClick={onConfirm}>
            {label}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

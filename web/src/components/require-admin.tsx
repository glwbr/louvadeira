import { useQuery } from "@tanstack/react-query"
import { LockKeyhole } from "lucide-react"
import type { ReactNode } from "react"

import { Skeleton } from "@/components/ui/skeleton"
import { ApiError, api, queryKeys } from "@/lib/api"

export function RequireAdmin({ children }: { children: ReactNode }) {
  const me = useQuery({ queryKey: queryKeys.me, queryFn: api.me })

  if (me.isPending) {
    return (
      <div className="mx-auto flex max-w-2xl flex-col gap-3 px-4 py-8">
        <Skeleton className="h-10 w-40" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-24 w-full" />
      </div>
    )
  }
  if (me.isError) {
    const unauthorized = me.error instanceof ApiError && me.error.status === 401
    return (
      <AccessMessage
        title={unauthorized ? "Acesso restrito" : "Não foi possível abrir o app"}
        message={
          unauthorized
            ? "Peça ao administrador do ministério um link de acesso. O link chega pelo WhatsApp e vale por 15 minutos."
            : me.error.message
        }
      />
    )
  }
  if (!me.data.isAdmin) {
    return (
      <AccessMessage
        title={`Olá, ${me.data.name}`}
        message="Por enquanto só os administradores usam esta área. Se precisar mudar algo no cadastro, peça a um deles."
      />
    )
  }
  return children
}

function AccessMessage({ title, message }: { title: string; message: string }) {
  return (
    <main className="mx-auto flex min-h-svh max-w-sm flex-col items-center justify-center gap-4 px-6 text-center">
      <div className="flex size-14 items-center justify-center rounded-2xl bg-primary/15 text-primary">
        <LockKeyhole className="size-6" />
      </div>
      <h1 className="text-xl font-semibold">{title}</h1>
      <p className="text-sm text-muted-foreground">{message}</p>
    </main>
  )
}

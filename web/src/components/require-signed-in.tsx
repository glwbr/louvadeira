import { useQuery } from "@tanstack/react-query"
import type { ReactNode } from "react"
import { Navigate } from "react-router"

import { Skeleton } from "@/components/ui/skeleton"
import { ApiError, api, queryKeys, type Person } from "@/lib/api"

/**
 * NOTE: Renders children for whoever is signed in; anyone else goes to the sign-in page, where they can ask for a
 * link on WhatsApp.
 */
export function RequireSignedIn({ children }: { children: (me: Person) => ReactNode }) {
  const me = useQuery({ queryKey: queryKeys.me, queryFn: api.me, retry: false })

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
    if (me.error instanceof ApiError && me.error.status === 401) {
      return <Navigate to="/entrar" replace />
    }
    return (
      <main className="mx-auto flex min-h-svh max-w-sm flex-col items-center justify-center gap-3 px-6 text-center">
        <h1 className="text-xl font-semibold">Não deu para abrir o app</h1>
        <p className="text-sm text-muted-foreground">{me.error.message}</p>
      </main>
    )
  }
  return children(me.data)
}

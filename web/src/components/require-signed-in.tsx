import { useQuery } from "@tanstack/react-query"
import type { ReactNode } from "react"
import { Navigate } from "react-router"

import { Wordmark } from "@/components/brand"
import { ErrorState } from "@/components/error-state"
import { CardSkeleton } from "@/components/list-skeleton"
import { Skeleton } from "@/components/ui/skeleton"
import { ApiError, api, queryKeys, type Person } from "@/lib/api"

/**
 * NOTE: Renders children for whoever is signed in; anyone else goes to the sign-in page, where they can ask for a
 * link on WhatsApp. While loading it draws the same header the app will have, so nothing jumps.
 */
export function RequireSignedIn({ children }: { children: (me: Person) => ReactNode }) {
  const me = useQuery({ queryKey: queryKeys.me, queryFn: api.me, retry: false })

  if (me.isPending) {
    return (
      <Frame>
        <Skeleton className="mb-6 h-9 w-48" />
        <div className="flex flex-col gap-3">
          <CardSkeleton />
          <CardSkeleton />
        </div>
      </Frame>
    )
  }
  if (me.isError) {
    if (me.error instanceof ApiError && me.error.status === 401) {
      return <Navigate to="/entrar" replace />
    }
    return (
      <Frame>
        <ErrorState error={me.error} onRetry={() => void me.refetch()} retrying={me.isFetching} />
      </Frame>
    )
  }
  return children(me.data)
}

function Frame({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-svh">
      <header className="border-b">
        <div className="mx-auto flex h-16 max-w-3xl items-center px-4">
          <Wordmark />
        </div>
      </header>
      <main className="mx-auto max-w-3xl px-4 pt-6">{children}</main>
    </div>
  )
}

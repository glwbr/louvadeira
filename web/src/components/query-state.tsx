import type { UseQueryResult } from "@tanstack/react-query"
import type { ReactNode } from "react"

import { ErrorState } from "@/components/error-state"

type Props<T> = {
  query: UseQueryResult<T>
  /** NOTE: Shown only while nothing has loaded yet; it waits 200ms itself, so fast answers never flash it. */
  loading: ReactNode
  empty?: ReactNode
  isEmpty?: (data: T) => boolean
  children: (data: T) => ReactNode
}

/**
 * INVARIANT: The order is loading, then error, then empty, then data, so a failed request never reads as "nothing
 * here" and a refetch keeps showing what is already on screen.
 */
export function QueryState<T>({ query, loading, empty, isEmpty, children }: Props<T>) {
  if (query.isPending) {
    return loading
  }
  if (query.isError && query.data === undefined) {
    return <ErrorState error={query.error} onRetry={() => void query.refetch()} retrying={query.isFetching} />
  }
  const data = query.data as T
  if (empty !== undefined && isEmpty?.(data)) {
    return empty
  }
  return children(data)
}

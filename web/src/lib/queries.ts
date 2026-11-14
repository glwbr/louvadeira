import { queryOptions, type QueryClient } from "@tanstack/react-query"

import { ApiError, api, queryKeys } from "@/lib/api"
import { monthBounds, nextMonth, shiftMonth, thisMonth } from "@/lib/format"

/**
 * NOTE: A month nobody generated yet answers 404. As data it is null, not an error, so the month kept on screen
 * while the next one loads is always the one the person was looking at, never an older month that did load.
 */
async function orNull<T>(request: Promise<T>): Promise<T | null> {
  try {
    return await request
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      return null
    }
    throw error
  }
}

/**
 * NOTE: The app's queries in one place, so a screen and the prefetch that warms it up always share the same cache.
 */
export const queries = {
  health: () => queryOptions({ queryKey: queryKeys.health, queryFn: api.health }),
  me: () => queryOptions({ queryKey: queryKeys.me, queryFn: api.me }),
  ministries: () => queryOptions({ queryKey: queryKeys.ministries, queryFn: api.ministries.list }),
  ministry: (id: number) => queryOptions({ queryKey: queryKeys.ministry(id), queryFn: () => api.ministries.get(id) }),
  people: () => queryOptions({ queryKey: queryKeys.people, queryFn: api.people.list }),
  services: () => queryOptions({ queryKey: queryKeys.services, queryFn: api.services.list }),
  schedule: (ministryId: number, month: string) =>
    queryOptions({
      queryKey: queryKeys.schedule(ministryId, month),
      queryFn: () => orNull(api.schedules.get(ministryId, month)),
    }),
  covers: (ministryId: number, month: string) =>
    queryOptions({
      queryKey: queryKeys.covers(ministryId, month),
      queryFn: () => api.schedules.covers(ministryId, month),
    }),
  unavailabilities: (month: string) => {
    const { from, to } = monthBounds(month)
    return queryOptions({
      queryKey: queryKeys.unavailabilities(from, to),
      queryFn: () => api.unavailabilities.list(from, to),
    })
  },
}

/**
 * NOTE: The months the home screen looks through for the next services.
 */
export function upcomingMonths(): string[] {
  return [thisMonth(), shiftMonth(thisMonth(), 1)]
}

/**
 * NOTE: Everything one month of the schedules screen shows, so paging to it draws at once.
 */
export function prefetchMonth(client: QueryClient, ministryId: number, month: string) {
  void client.prefetchQuery(queries.schedule(ministryId, month))
  void client.prefetchQuery(queries.covers(ministryId, month))
  void client.prefetchQuery(queries.unavailabilities(month))
}

/**
 * NOTE: Warms up the data of a screen before it opens: on idle after sign-in and as soon as a finger or pointer
 * reaches its tab. Fresh data is not fetched again, and a failure here stays silent: the screen asks again and shows
 * the error itself.
 */
export async function prefetchScreen(client: QueryClient, path: string) {
  switch (path) {
    case "/": {
      void client.prefetchQuery(queries.health())
      void client.prefetchQuery(queries.me())
      void client.prefetchQuery(queries.people())
      void client.prefetchQuery(queries.services())
      const ministries = await client.ensureQueryData(queries.ministries()).catch(() => [])
      for (const ministry of ministries) {
        for (const month of upcomingMonths()) {
          void client.prefetchQuery(queries.schedule(ministry.id, month))
        }
      }
      return
    }
    case "/escalas": {
      const ministries = await client.ensureQueryData(queries.ministries()).catch(() => [])
      const first = ministries[0]
      if (first) {
        void client.prefetchQuery(queries.ministry(first.id))
        prefetchMonth(client, first.id, nextMonth())
      }
      return
    }
    case "/ministerios":
      void client.prefetchQuery(queries.ministries())
      return
    case "/pessoas":
      void client.prefetchQuery(queries.people())
      void client.prefetchQuery(queries.me())
      return
    case "/cultos":
      void client.prefetchQuery(queries.services())
      return
  }
}

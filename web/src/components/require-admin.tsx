import type { ReactNode } from "react"
import { Navigate } from "react-router"

import { RequireSignedIn } from "@/components/require-signed-in"

/**
 * NOTE: Members who are not administrators land on their own page instead of a locked door.
 */
export function RequireAdmin({ children }: { children: ReactNode }) {
  return <RequireSignedIn>{(me) => (me.isAdmin ? children : <Navigate to="/minha-escala" replace />)}</RequireSignedIn>
}

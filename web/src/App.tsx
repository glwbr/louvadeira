import { lazy, Suspense, type ReactNode } from "react"
import { Navigate, Route, Routes } from "react-router"

import { AppShell } from "@/components/app-shell"
import { CardSkeleton } from "@/components/list-skeleton"
import { MemberShell } from "@/components/member-shell"
import { RequireAdmin } from "@/components/require-admin"
import { RequireSignedIn } from "@/components/require-signed-in"
import { Skeleton } from "@/components/ui/skeleton"

/**
 * TRADEOFF: Each screen loads on demand, so a member opening their link from WhatsApp does not download the leader's
 * screens first; the service worker still caches them all for offline use afterwards.
 */
const SignInPage = lazy(() => import("@/features/auth/sign-in-page").then((m) => ({ default: m.SignInPage })))
const HomePage = lazy(() => import("@/features/home/home-page").then((m) => ({ default: m.HomePage })))
const MemberPage = lazy(() => import("@/features/member/member-page").then((m) => ({ default: m.MemberPage })))
const MinistriesPage = lazy(() => import("@/features/ministries/ministries-page").then((m) => ({ default: m.MinistriesPage })))
const MinistryPage = lazy(() => import("@/features/ministries/ministry-page").then((m) => ({ default: m.MinistryPage })))
const PeoplePage = lazy(() => import("@/features/people/people-page").then((m) => ({ default: m.PeoplePage })))
const SchedulesPage = lazy(() => import("@/features/schedules/schedules-page").then((m) => ({ default: m.SchedulesPage })))
const ServicesPage = lazy(() => import("@/features/services/services-page").then((m) => ({ default: m.ServicesPage })))

function Page({ children }: { children: ReactNode }) {
  return (
    <Suspense
      fallback={
        <div className="flex flex-col gap-3">
          <Skeleton className="mb-3 h-9 w-48" />
          <CardSkeleton />
          <CardSkeleton />
        </div>
      }
    >
      {children}
    </Suspense>
  )
}

export default function App() {
  return (
    <Routes>
      <Route
        path="/entrar"
        element={
          <Suspense fallback={null}>
            <SignInPage />
          </Suspense>
        }
      />
      <Route element={<RequireSignedIn>{(me) => <MemberShell me={me} />}</RequireSignedIn>}>
        <Route path="minha-escala" element={<Page><MemberPage /></Page>} />
      </Route>
      <Route
        element={
          <RequireAdmin>
            <AppShell />
          </RequireAdmin>
        }
      >
        <Route index element={<Page><HomePage /></Page>} />
        <Route path="escalas" element={<Page><SchedulesPage /></Page>} />
        <Route path="ministerios" element={<Page><MinistriesPage /></Page>} />
        <Route path="ministerios/:id" element={<Page><MinistryPage /></Page>} />
        <Route path="pessoas" element={<Page><PeoplePage /></Page>} />
        <Route path="cultos" element={<Page><ServicesPage /></Page>} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

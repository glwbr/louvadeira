import { Suspense, type ReactNode } from "react"
import { Navigate, Route, Routes } from "react-router"

import { AppShell } from "@/components/app-shell"
import { CardSkeleton } from "@/components/list-skeleton"
import { MemberShell } from "@/components/member-shell"
import { RequireAdmin } from "@/components/require-admin"
import { RequireSignedIn } from "@/components/require-signed-in"
import { Skeleton } from "@/components/ui/skeleton"
import { whenIdle } from "@/lib/idle"
import { preloadable } from "@/lib/preloadable"

/**
 * TRADEOFF: Each screen is its own file, so a member opening their link from WhatsApp gets their page first. Once the
 * app is idle the other screens download in the background, so moving between tabs never waits on a file.
 */
const screens = {
  SignInPage: preloadable(() => import("@/features/auth/sign-in-page").then((m) => ({ default: m.SignInPage }))),
  HomePage: preloadable(() => import("@/features/home/home-page").then((m) => ({ default: m.HomePage }))),
  MemberPage: preloadable(() => import("@/features/member/member-page").then((m) => ({ default: m.MemberPage }))),
  MinistriesPage: preloadable(() => import("@/features/ministries/ministries-page").then((m) => ({ default: m.MinistriesPage }))),
  MinistryPage: preloadable(() => import("@/features/ministries/ministry-page").then((m) => ({ default: m.MinistryPage }))),
  PeoplePage: preloadable(() => import("@/features/people/people-page").then((m) => ({ default: m.PeoplePage }))),
  SchedulesPage: preloadable(() => import("@/features/schedules/schedules-page").then((m) => ({ default: m.SchedulesPage }))),
  ServicesPage: preloadable(() => import("@/features/services/services-page").then((m) => ({ default: m.ServicesPage }))),
}
const { SignInPage, HomePage, MemberPage, MinistriesPage, MinistryPage, PeoplePage, SchedulesPage, ServicesPage } = screens

whenIdle(() => Object.values(screens).forEach((screen) => void screen.preload()))

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

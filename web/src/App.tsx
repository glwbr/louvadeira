import { Navigate, Route, Routes } from "react-router"

import { AppShell } from "@/components/app-shell"
import { MemberShell } from "@/components/member-shell"
import { RequireAdmin } from "@/components/require-admin"
import { RequireSignedIn } from "@/components/require-signed-in"
import { SignInPage } from "@/features/auth/sign-in-page"
import { HomePage } from "@/features/home/home-page"
import { MemberPage } from "@/features/member/member-page"
import { MinistriesPage } from "@/features/ministries/ministries-page"
import { MinistryPage } from "@/features/ministries/ministry-page"
import { PeoplePage } from "@/features/people/people-page"
import { SchedulesPage } from "@/features/schedules/schedules-page"
import { ServicesPage } from "@/features/services/services-page"

export default function App() {
  return (
    <Routes>
      <Route path="/entrar" element={<SignInPage />} />
      <Route element={<RequireSignedIn>{(me) => <MemberShell me={me} />}</RequireSignedIn>}>
        <Route path="minha-escala" element={<MemberPage />} />
      </Route>
      <Route
        element={
          <RequireAdmin>
            <AppShell />
          </RequireAdmin>
        }
      >
        <Route index element={<HomePage />} />
        <Route path="escalas" element={<SchedulesPage />} />
        <Route path="ministerios" element={<MinistriesPage />} />
        <Route path="ministerios/:id" element={<MinistryPage />} />
        <Route path="pessoas" element={<PeoplePage />} />
        <Route path="cultos" element={<ServicesPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

import { useQueryClient } from "@tanstack/react-query"
import { LayoutDashboard, LogOut } from "lucide-react"
import { Link, Outlet } from "react-router"

import { Button } from "@/components/ui/button"
import { api, type Person } from "@/lib/api"

export function MemberShell({ me }: { me: Person }) {
  const queryClient = useQueryClient()

  const logout = async () => {
    await api.logout().catch(() => undefined)
    queryClient.clear()
    window.location.assign("/entrar")
  }

  return (
    <div className="min-h-svh pb-[env(safe-area-inset-bottom)]">
      <header className="sticky top-0 z-10 border-b bg-background/85 backdrop-blur">
        <div className="mx-auto flex h-14 max-w-2xl items-center justify-between px-4">
          <div className="flex items-center gap-2">
            <img src="/favicon.svg" alt="" className="size-7" />
            <span className="font-semibold tracking-tight">louvadeira</span>
          </div>
          <div className="flex items-center gap-1">
            {me.isAdmin && (
              <Button variant="ghost" size="sm" asChild>
                <Link to="/">
                  <LayoutDashboard />
                  Painel
                </Link>
              </Button>
            )}
            <Button variant="ghost" size="sm" onClick={logout}>
              <LogOut />
              Sair
            </Button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-2xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}

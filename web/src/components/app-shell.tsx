import { useQueryClient } from "@tanstack/react-query"
import { CalendarClock, CalendarDays, Church, Home, LogOut, Users } from "lucide-react"
import { NavLink, Outlet } from "react-router"

import { Button } from "@/components/ui/button"
import { api } from "@/lib/api"
import { cn } from "@/lib/utils"

const NAV = [
  { to: "/", label: "Início", icon: Home, end: true },
  { to: "/escalas", label: "Escalas", icon: CalendarDays, end: false },
  { to: "/ministerios", label: "Ministérios", icon: Church, end: false },
  { to: "/pessoas", label: "Pessoas", icon: Users, end: false },
  { to: "/cultos", label: "Cultos", icon: CalendarClock, end: false },
]

export function AppShell() {
  const queryClient = useQueryClient()

  const logout = async () => {
    await api.logout().catch(() => undefined)
    queryClient.clear()
    window.location.assign("/")
  }

  return (
    <div className="min-h-svh pb-[calc(4.5rem+env(safe-area-inset-bottom))]">
      <header className="sticky top-0 z-10 border-b bg-background/85 backdrop-blur">
        <div className="mx-auto flex h-14 max-w-2xl items-center justify-between px-4">
          <div className="flex items-center gap-2">
            <img src="/favicon.svg" alt="" className="size-7" />
            <span className="font-semibold tracking-tight">louvadeira</span>
          </div>
          <Button variant="ghost" size="sm" onClick={logout}>
            <LogOut />
            Sair
          </Button>
        </div>
      </header>

      <main className="mx-auto max-w-2xl px-4 py-6">
        <Outlet />
      </main>

      <nav className="fixed inset-x-0 bottom-0 z-10 border-t bg-background/95 pb-[env(safe-area-inset-bottom)] backdrop-blur">
        <div className="mx-auto grid h-16 max-w-2xl grid-cols-5">
          {NAV.map(({ to, label, icon: Icon, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                cn(
                  "flex flex-col items-center justify-center gap-1 text-xs text-muted-foreground transition-colors",
                  isActive && "text-primary"
                )
              }
            >
              <Icon className="size-5" />
              {label}
            </NavLink>
          ))}
        </div>
      </nav>
    </div>
  )
}

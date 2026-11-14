import { useQueryClient } from "@tanstack/react-query"
import { CalendarClock, CalendarDays, Church, Home, Users, type LucideIcon } from "lucide-react"
import { motion } from "motion/react"
import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { Link, NavLink, useLocation, useOutlet } from "react-router"

import { Wordmark } from "@/components/brand"
import { ScreenTransition, spring } from "@/components/motion"
import { ActivityBar, SignOutButton } from "@/components/shell-parts"
import { whenIdle } from "@/lib/idle"
import { prefetchScreen } from "@/lib/queries"
import { cn } from "@/lib/utils"

type NavItem = { to: string; label: "nav.home" | "nav.schedules" | "nav.ministries" | "nav.people" | "nav.services"; icon: LucideIcon; end: boolean }

const NAV: NavItem[] = [
  { to: "/", label: "nav.home", icon: Home, end: true },
  { to: "/escalas", label: "nav.schedules", icon: CalendarDays, end: false },
  { to: "/ministerios", label: "nav.ministries", icon: Church, end: false },
  { to: "/pessoas", label: "nav.people", icon: Users, end: false },
  { to: "/cultos", label: "nav.services", icon: CalendarClock, end: false },
]

export function AppShell() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const location = useLocation()
  const outlet = useOutlet()
  // NOTE: A screen's data is fetched while the finger is still on its tab, so it opens drawn instead of filling in.
  const warm = (to: string) => () => void prefetchScreen(queryClient, to)

  // NOTE: Once the first screen settles, the others are warmed up too, so moving between tabs is instant.
  useEffect(() => whenIdle(() => NAV.forEach(({ to }) => void prefetchScreen(queryClient, to))), [queryClient])

  return (
    <div className="min-h-svh pb-[calc(5rem+env(safe-area-inset-bottom))] lg:pb-12">
      <header className="sticky top-0 z-20 border-b bg-background/85 backdrop-blur-md">
        <ActivityBar />
        <div className="mx-auto flex h-16 max-w-3xl items-center gap-4 px-4 lg:max-w-4xl">
          <Link to="/" className="rounded-lg focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none">
            <Wordmark />
          </Link>
          <nav aria-label={t("nav.label")} className="ml-2 hidden flex-1 items-center gap-1 lg:flex">
            {NAV.map(({ to, label, icon: Icon, end }) => (
              <NavLink
                key={to}
                to={to}
                end={end}
                onPointerEnter={warm(to)}
                onFocus={warm(to)}
                className="relative flex h-9 items-center gap-2 rounded-lg px-3 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground aria-[current=page]:text-foreground"
              >
                {({ isActive }) => (
                  <>
                    {isActive && (
                      <motion.span layoutId="top-nav-pill" transition={spring} className="absolute inset-0 rounded-lg bg-primary/15" />
                    )}
                    <Icon className={cn("relative size-4", isActive && "text-brand")} />
                    <span className="relative">{t(label)}</span>
                  </>
                )}
              </NavLink>
            ))}
          </nav>
          <div className="ml-auto">
            <SignOutButton to="/" />
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-3xl px-4 pt-6 pb-8 lg:max-w-4xl lg:pt-10">
        <ScreenTransition id={location.pathname}>{outlet}</ScreenTransition>
      </main>

      <nav
        aria-label={t("nav.label")}
        className="fixed inset-x-0 bottom-0 z-20 border-t bg-background/90 pb-[env(safe-area-inset-bottom)] backdrop-blur-md lg:hidden"
      >
        <div className="mx-auto grid h-[4.25rem] max-w-3xl grid-cols-5">
          {NAV.map(({ to, label, icon: Icon, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              onPointerDown={warm(to)}
              onFocus={warm(to)}
              className="group flex flex-col items-center justify-center gap-1 text-[11px] font-medium text-muted-foreground transition-colors aria-[current=page]:text-foreground"
            >
              {({ isActive }) => (
                <>
                  <span className="relative flex h-8 w-14 items-center justify-center">
                    {isActive && (
                      <motion.span layoutId="bottom-nav-pill" transition={spring} className="absolute inset-0 rounded-full bg-primary/18" />
                    )}
                    <Icon className={cn("relative size-5 transition-transform group-active:scale-90", isActive && "text-brand")} strokeWidth={isActive ? 2.2 : 1.8} />
                  </span>
                  <span className="max-w-full truncate px-0.5">{t(label)}</span>
                </>
              )}
            </NavLink>
          ))}
        </div>
      </nav>
    </div>
  )
}

import { LayoutDashboard } from "lucide-react"
import { motion } from "motion/react"
import { useTranslation } from "react-i18next"
import { Link, Outlet } from "react-router"

import { Wordmark } from "@/components/brand"
import { quick } from "@/components/motion"
import { ActivityBar, SignOutButton } from "@/components/shell-parts"
import { Button } from "@/components/ui/button"
import type { Person } from "@/lib/api"

export function MemberShell({ me }: { me: Person }) {
  const { t } = useTranslation()
  return (
    <div className="min-h-svh pb-[calc(2rem+env(safe-area-inset-bottom))]">
      <header className="sticky top-0 z-20 border-b bg-background/85 backdrop-blur-md">
        <ActivityBar />
        <div className="mx-auto flex h-16 max-w-2xl items-center justify-between gap-3 px-4">
          <Wordmark />
          <div className="flex items-center gap-1">
            {me.isAdmin && (
              <Button variant="ghost" size="sm" className="text-muted-foreground hover:text-foreground" asChild>
                <Link to="/">
                  <LayoutDashboard />
                  {t("nav.panel")}
                </Link>
              </Button>
            )}
            <SignOutButton to="/entrar" />
          </div>
        </div>
      </header>
      <motion.main
        initial={{ opacity: 0, y: 6 }}
        animate={{ opacity: 1, y: 0 }}
        transition={quick}
        className="mx-auto max-w-2xl px-4 pt-6 pb-8"
      >
        <Outlet />
      </motion.main>
    </div>
  )
}

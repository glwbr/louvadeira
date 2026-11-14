import { useQuery } from "@tanstack/react-query"
import { ChevronRight, Church, Plus } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import { EmptyState } from "@/components/empty-state"
import { ErrorState } from "@/components/error-state"
import { ListSkeleton } from "@/components/list-skeleton"
import { RevealItem } from "@/components/motion"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { api, queryKeys } from "@/lib/api"
import { rotationLabel } from "@/lib/format"

import { MinistryDialog } from "./ministry-dialog"

export function MinistriesPage() {
  const { t } = useTranslation()
  const ministries = useQuery({ queryKey: queryKeys.ministries, queryFn: api.ministries.list })
  const [open, setOpen] = useState(false)

  const add = (
    <Button onClick={() => setOpen(true)}>
      <Plus />
      {t("ministries.new")}
    </Button>
  )

  return (
    <>
      <PageHeader title={t("ministries.title")} description={t("ministries.lead")} action={add} />

      {ministries.isPending ? (
        <ListSkeleton />
      ) : ministries.isError ? (
        <ErrorState error={ministries.error} onRetry={() => void ministries.refetch()} retrying={ministries.isFetching} />
      ) : ministries.data.length === 0 ? (
        <EmptyState icon={Church} title={t("ministries.emptyTitle")}>
          {t("ministries.empty")}
        </EmptyState>
      ) : (
        <ul className="flex flex-col gap-2.5">
          {ministries.data.map((ministry, index) => (
            <RevealItem key={ministry.id} index={index}>
              <Link
                to={`/ministerios/${ministry.id}`}
                className="group flex items-center gap-3.5 rounded-2xl border bg-card p-4 transition-colors hover:bg-accent/40"
              >
                <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-primary/12 text-brand">
                  <Church className="size-5" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block font-medium text-pretty">{ministry.name}</span>
                  <span className="block text-sm text-muted-foreground">
                    {t("ministries.people", { count: ministry.memberCount })} · {rotationLabel(ministry.rotationUnit)}
                  </span>
                </span>
                <ChevronRight className="size-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
              </Link>
            </RevealItem>
          ))}
        </ul>
      )}

      <MinistryDialog open={open} onOpenChange={setOpen} />
    </>
  )
}

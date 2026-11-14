import { useQuery } from "@tanstack/react-query"
import { CalendarClock, ChevronRight, Grape, Plus } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import { CommunionTag } from "@/components/communion-tag"
import { EmptyState } from "@/components/empty-state"
import { ErrorState } from "@/components/error-state"
import { ListSkeleton } from "@/components/list-skeleton"
import { RevealItem } from "@/components/motion"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { api, queryKeys, type Service } from "@/lib/api"
import { communionLabel, formatTime, ordinalOfMonth, serviceLabel, weekdayShortName } from "@/lib/format"
import { cn } from "@/lib/utils"

import { CommunionDialog } from "./communion-dialog"
import { ServiceDialog } from "./service-dialog"

export function ServicesPage() {
  const { t } = useTranslation()
  const services = useQuery({ queryKey: queryKeys.services, queryFn: api.services.list })
  const [editing, setEditing] = useState<Service | undefined>()
  const [open, setOpen] = useState(false)
  const [choosingCommunion, setChoosingCommunion] = useState(false)

  const openDialog = (service?: Service) => {
    setEditing(service)
    setOpen(true)
  }
  const communion = services.data?.find((service) => service.communionWeek)
  const sorted = [...(services.data ?? [])].sort(
    (a, b) => Number(b.active) - Number(a.active) || a.weekday - b.weekday || a.startTime.localeCompare(b.startTime)
  )

  return (
    <>
      <PageHeader
        title={t("services.title")}
        description={t("services.lead")}
        action={
          <Button onClick={() => openDialog()}>
            <Plus />
            {t("services.new")}
          </Button>
        }
      />

      {services.data && services.data.length > 0 && (
        <section aria-labelledby="communion" className="mb-6 flex items-center gap-3.5 rounded-2xl border bg-card p-4">
          <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-primary/12 text-brand">
            <Grape className="size-5" />
          </span>
          <div className="min-w-0 flex-1">
            <h2 id="communion" className="font-medium">
              {t("services.communion.title")}
            </h2>
            <p className="text-sm text-pretty text-muted-foreground">
              {communion?.communionWeek
                ? t("services.communion.current", {
                    when: ordinalOfMonth(communion.communionWeek, communion.weekday),
                    service: communion.name,
                  })
                : t("services.communion.none")}
            </p>
          </div>
          <Button size="sm" variant="outline" className="shrink-0" onClick={() => setChoosingCommunion(true)}>
            {communion ? t("services.communion.change") : t("services.communion.choose")}
          </Button>
        </section>
      )}

      {services.isPending ? (
        <ListSkeleton />
      ) : services.isError ? (
        <ErrorState error={services.error} onRetry={() => void services.refetch()} retrying={services.isFetching} />
      ) : sorted.length === 0 ? (
        <EmptyState icon={CalendarClock} title={t("services.emptyTitle")}>
          {t("services.empty")}
        </EmptyState>
      ) : (
        <ul className="flex flex-col divide-y rounded-2xl border bg-card">
          {sorted.map((service, index) => (
            <RevealItem key={service.id} index={index}>
              <button
                type="button"
                onClick={() => openDialog(service)}
                className="group flex w-full items-center gap-3.5 px-4 py-3.5 text-left transition-colors hover:bg-accent/40"
              >
                <span
                  aria-hidden
                  className={cn(
                    "flex w-12 shrink-0 flex-col items-center rounded-xl border bg-background/40 py-1.5 leading-none",
                    !service.active && "opacity-50"
                  )}
                >
                  <span className="text-[0.6875rem] font-semibold tracking-wide text-brand uppercase">{weekdayShortName(service.weekday)}</span>
                  <span className="tabular mt-1 text-sm font-semibold">{formatTime(service.startTime)}</span>
                </span>
                <span className={cn("min-w-0 flex-1", !service.active && "opacity-60")}>
                  <span className="block font-medium text-pretty">{service.name}</span>
                  <span className="block text-sm text-muted-foreground">{serviceLabel(service)}</span>
                  {communionLabel(service) && <CommunionTag className="mt-1" label={communionLabel(service)} />}
                </span>
                {!service.active && <Badge variant="secondary">{t("services.inactive")}</Badge>}
                <ChevronRight className="size-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
              </button>
            </RevealItem>
          ))}
        </ul>
      )}

      <ServiceDialog open={open} onOpenChange={setOpen} service={editing} />
      <CommunionDialog
        open={choosingCommunion}
        onOpenChange={setChoosingCommunion}
        services={sorted.filter((service) => service.active)}
        current={communion}
      />
    </>
  )
}

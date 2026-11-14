import { useQuery } from "@tanstack/react-query"
import { CalendarClock, Grape, Plus } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import { CommunionTag } from "@/components/communion-tag"
import { EmptyState } from "@/components/empty-state"
import { ListSkeleton } from "@/components/list-skeleton"
import { IconTile } from "@/components/icon-tile"
import { ListCard, ListRow } from "@/components/list"
import { AnimatedItem } from "@/components/motion"
import { QueryState } from "@/components/query-state"
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
          <IconTile icon={Grape} />
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

      <QueryState
        query={services}
        loading={<ListSkeleton />}
        isEmpty={(data) => data.length === 0}
        empty={
          <EmptyState icon={CalendarClock} title={t("services.emptyTitle")}>
            {t("services.empty")}
          </EmptyState>
        }
      >
        {() => (
          <ListCard>
            {sorted.map((service) => (
              <AnimatedItem key={service.id}>
                <ListRow
                  onClick={() => openDialog(service)}
                  className={cn("py-3.5", !service.active && "[&>span:not(:last-of-type)]:opacity-60")}
                  leading={<WeekdayTile weekday={service.weekday} startTime={service.startTime} />}
                  title={service.name}
                  subtitle={
                    <>
                      {serviceLabel(service)}
                      {communionLabel(service) && <CommunionTag className="mt-1 flex w-fit" label={communionLabel(service)} />}
                    </>
                  }
                  trailing={!service.active && <Badge variant="secondary">{t("services.inactive")}</Badge>}
                />
              </AnimatedItem>
            ))}
          </ListCard>
        )}
      </QueryState>

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

function WeekdayTile({ weekday, startTime }: { weekday: number; startTime: string }) {
  return (
    <span aria-hidden className="flex w-12 shrink-0 flex-col items-center rounded-xl border bg-background/40 py-1.5 leading-none">
      <span className="text-[0.6875rem] font-semibold tracking-wide text-brand uppercase">{weekdayShortName(weekday)}</span>
      <span className="tabular mt-1 text-sm font-semibold">{formatTime(startTime)}</span>
    </span>
  )
}

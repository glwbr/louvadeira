import { useQueries, useQuery } from "@tanstack/react-query"
import { CalendarClock, CalendarDays, CalendarHeart, ChevronRight, Church, Users, type LucideIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import { CommunionTag } from "@/components/communion-tag"
import { DateBlock } from "@/components/date-block"
import { EmptyState } from "@/components/empty-state"
import { ListRow } from "@/components/list"
import { ErrorState } from "@/components/error-state"
import { CardSkeleton } from "@/components/list-skeleton"
import { AnimatedItem, AnimatedList } from "@/components/motion"
import { RoleIcon } from "@/components/role-icon"
import { SlotStatusIcon } from "@/components/slot-status"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { ApiError, api, queryKeys, type Health, type Ministry, type ScheduleService } from "@/lib/api"
import { firstName, longDate, shiftMonth, thisMonth, todayISO } from "@/lib/format"
import { cn } from "@/lib/utils"

const UPCOMING_LIMIT = 3

export function HomePage() {
  const { t } = useTranslation()
  const me = useQuery({ queryKey: queryKeys.me, queryFn: api.me })
  const ministries = useQuery({ queryKey: queryKeys.ministries, queryFn: api.ministries.list })
  const people = useQuery({ queryKey: queryKeys.people, queryFn: api.people.list })
  const services = useQuery({ queryKey: queryKeys.services, queryFn: api.services.list })

  return (
    <div className="flex flex-col gap-8">
      <div>
        <h1 className="font-display text-[2rem] leading-[1.1] font-semibold text-balance">
          {me.data ? t("home.greeting", { name: firstName(me.data.name) }) : <Skeleton className="h-9 w-44" />}
        </h1>
        <p className="mt-1.5 text-sm text-muted-foreground first-letter:uppercase">{longDate(todayISO())}</p>
      </div>

      <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_19rem] lg:items-start">
        <section aria-labelledby="upcoming" className="flex min-w-0 flex-col gap-3">
          <div className="flex items-baseline justify-between gap-3">
            <h2 id="upcoming" className="text-lg font-semibold">
              {t("home.upcoming")}
            </h2>
            <Link to="/escalas" className="-m-2 p-2 text-sm font-medium text-brand hover:underline">
              {t("home.seeSchedules")}
            </Link>
          </div>
          {ministries.isPending ? (
            <UpcomingSkeleton />
          ) : ministries.isError ? (
            <ErrorState error={ministries.error} onRetry={() => void ministries.refetch()} retrying={ministries.isFetching} />
          ) : (
            <Upcoming ministries={ministries.data} />
          )}
        </section>

        <aside className="flex flex-col gap-8">
          <Link
            to="/minha-escala"
            className="group flex items-center gap-3 rounded-2xl border bg-card px-4 py-3.5 transition-colors hover:bg-accent/50"
          >
            <span className="flex size-10 items-center justify-center rounded-xl bg-primary/12 text-brand">
              <CalendarHeart className="size-5" />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block font-medium">{t("home.mySchedules")}</span>
              <span className="block text-xs text-muted-foreground">{t("home.mySchedulesHint")}</span>
            </span>
            <ChevronRight className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
          </Link>

          <section aria-labelledby="registry" className="flex flex-col gap-3">
            <h2 id="registry" className="text-lg font-semibold">
              {t("home.registry")}
            </h2>
            <ul className="divide-y overflow-hidden rounded-2xl border bg-card">
              <RegistryRow to="/ministerios" icon={Church} label={t("nav.ministries")} count={ministries.data?.length} />
              <RegistryRow to="/pessoas" icon={Users} label={t("nav.people")} count={people.data?.length} />
              <RegistryRow
                to="/cultos"
                icon={CalendarClock}
                label={t("nav.services")}
                count={services.data?.filter((s) => s.active).length}
              />
            </ul>
          </section>

          <WhatsAppStatus />
        </aside>
      </div>
    </div>
  )
}

type Upcoming = { ministry: Ministry; month: string; service: ScheduleService }

function Upcoming({ ministries }: { ministries: Ministry[] }) {
  const { t } = useTranslation()
  const months = [thisMonth(), shiftMonth(thisMonth(), 1)]
  const pairs = ministries.flatMap((ministry) => months.map((month) => ({ ministry, month })))
  const schedules = useQueries({
    queries: pairs.map(({ ministry, month }) => ({
      queryKey: [...queryKeys.schedule(ministry.id, month), "upcoming"],
      // NOTE: A month without a schedule is normal here, not an error.
      queryFn: () =>
        api.schedules.get(ministry.id, month).catch((error: unknown) => {
          if (error instanceof ApiError && error.status === 404) {
            return null
          }
          throw error
        }),
    })),
  })

  if (schedules.some((query) => query.isPending)) {
    return <UpcomingSkeleton />
  }
  const failed = schedules.find((query) => query.isError)
  if (failed) {
    return (
      <ErrorState
        error={failed.error}
        onRetry={() => schedules.forEach((query) => query.isError && void query.refetch())}
        retrying={schedules.some((query) => query.isFetching)}
      />
    )
  }

  const today = todayISO()
  const upcoming: Upcoming[] = schedules
    .flatMap((query, index) => {
      const view = query.data
      if (!view || view.status !== "published") {
        return []
      }
      return view.weeks.flatMap((week) =>
        week.services.filter((service) => service.date >= today).map((service) => ({ ...pairs[index], service }))
      )
    })
    .sort((a, b) => (a.service.date + a.service.startTime).localeCompare(b.service.date + b.service.startTime))
    .slice(0, UPCOMING_LIMIT)

  if (upcoming.length === 0) {
    return (
      <EmptyState
        icon={CalendarDays}
        title={t("home.noUpcomingTitle")}
        action={
          <Link to="/escalas" className="text-sm font-medium text-brand hover:underline">
            {t("home.openSchedules")}
          </Link>
        }
      >
        {t("home.noUpcoming")}
      </EmptyState>
    )
  }

  return (
    <ul className="flex flex-col gap-3">
      <AnimatedList>
        {upcoming.map((item) => (
          <AnimatedItem key={`${item.ministry.id}-${item.service.date}-${item.service.serviceId}`}>
            <ServiceCard item={item} showMinistry={ministries.length > 1} />
          </AnimatedItem>
        ))}
      </AnimatedList>
    </ul>
  )
}

function ServiceCard({ item, showMinistry }: { item: Upcoming; showMinistry: boolean }) {
  const { t } = useTranslation()
  const { service, ministry, month } = item
  const slots = service.roles.flatMap((role) => role.slots)
  const filled = slots.filter((slot) => slot.personId !== 0)
  const confirmed = filled.filter((slot) => slot.status === "confirmed").length
  const covers = slots.filter((slot) => slot.coverOpen).length
  const empty = slots.length - filled.length

  return (
    <Link
      to={`/escalas?mes=${month}&ministerio=${ministry.id}`}
      className="group flex gap-4 rounded-2xl border bg-card p-4 transition-colors hover:bg-accent/40"
    >
      <DateBlock date={service.date} startTime={service.startTime} />
      <div className="min-w-0 flex-1">
        <p className="font-medium text-pretty">
          {service.name}
          {showMinistry && <span className="text-muted-foreground"> · {ministry.name}</span>}
        </p>
        {service.communion && <CommunionTag className="mt-1" />}
        <ul className="mt-2.5 grid gap-1.5 text-sm">
          {service.roles.map((role) => {
            return (
              <li key={role.roleId} className="flex min-w-0 items-start gap-2">
                <RoleIcon name={role.name} className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-label={role.name} />
                <span className="flex min-w-0 flex-wrap gap-x-3 gap-y-1">
                  {role.slots.map((slot) => (
                    <span key={slot.assignmentId} className="inline-flex min-w-0 items-center gap-1.5">
                      {slot.personId === 0 ? (
                        <span className="text-danger">{t("home.vacant")}</span>
                      ) : (
                        <>
                          <SlotStatusIcon status={slot.status} coverOpen={slot.coverOpen} />
                          <span className={cn("truncate", slot.status === "declined" && !slot.coverOpen && "text-muted-foreground")}>
                            {firstName(slot.personName)}
                          </span>
                        </>
                      )}
                    </span>
                  ))}
                </span>
              </li>
            )
          })}
        </ul>
        <div className="mt-3 flex flex-wrap gap-1.5">
          <Badge variant={confirmed === filled.length && filled.length > 0 ? "success" : "secondary"}>
            {confirmed === 0
              ? t("home.noneConfirmed")
              : confirmed === filled.length
                ? t("home.allConfirmed")
                : t("home.confirmed", { count: confirmed, total: filled.length })}
          </Badge>
          {covers > 0 && <Badge variant="warning">{t("home.openCovers", { count: covers })}</Badge>}
          {empty > 0 && <Badge variant="destructive">{t("home.empty", { count: empty })}</Badge>}
        </div>
      </div>
      <ChevronRight className="mt-1 size-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
    </Link>
  )
}

function UpcomingSkeleton() {
  return (
    <div className="flex flex-col gap-3">
      <CardSkeleton lines={4} />
      <CardSkeleton lines={4} />
    </div>
  )
}

function RegistryRow({ to, icon: Icon, label, count }: { to: string; icon: LucideIcon; label: string; count?: number }) {
  return (
    <li>
      <ListRow
        to={to}
        leading={<Icon className="size-5 text-brand" aria-hidden />}
        title={label}
        trailing={count === undefined ? <Skeleton className="h-4 w-6" /> : <span className="tabular text-sm text-muted-foreground">{count}</span>}
      />
    </li>
  )
}

const MODES: Record<Health["whatsapp"]["mode"], "off" | "test" | "live" | "unknown"> = {
  off: "off",
  test: "test",
  live: "live",
  "": "unknown",
}

function WhatsAppStatus() {
  const { t } = useTranslation()
  const health = useQuery({
    queryKey: queryKeys.health,
    queryFn: api.health,
    refetchInterval: 10_000,
    meta: { background: true },
  })
  const mode = health.data ? MODES[health.data.whatsapp.mode] : undefined
  const live = health.data?.whatsapp.linked && health.data.whatsapp.connected

  return (
    <section aria-labelledby="whatsapp" className="flex flex-col gap-3">
      <h2 id="whatsapp" className="text-lg font-semibold">
        {t("home.whatsapp")}
      </h2>
      <div className="rounded-2xl border bg-card p-4">
        {health.isPending ? (
          <div className="flex flex-col gap-2">
            <Skeleton className="h-4 w-1/3" />
            <Skeleton className="h-3 w-3/4" />
          </div>
        ) : health.isError ? (
          <p className="flex items-center gap-2 text-sm font-medium text-danger">
            <span className="size-2 rounded-full bg-danger" />
            {t("home.serverDown")}
          </p>
        ) : (
          <>
            <p className="flex items-center gap-2 font-medium">
              <span className={cn("size-2 rounded-full", live ? "bg-success" : mode === "off" ? "bg-muted-foreground" : "bg-warning")} />
              {mode && t(`home.modeLabel.${mode}`)}
            </p>
            {mode && <p className="mt-1 text-sm text-pretty text-muted-foreground">{t(`home.mode.${mode}`)}</p>}
            <div className="mt-3 flex flex-wrap gap-1.5">
              <Badge variant={health.data.whatsapp.linked ? "success" : "secondary"}>
                {health.data.whatsapp.linked ? t("home.linked") : t("home.notLinked")}
              </Badge>
              {health.data.whatsapp.linked && (
                <Badge variant={health.data.whatsapp.connected ? "success" : "destructive"}>
                  {health.data.whatsapp.connected ? t("home.connected") : t("home.disconnected")}
                </Badge>
              )}
            </div>
          </>
        )}
      </div>
    </section>
  )
}

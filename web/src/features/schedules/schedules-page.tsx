import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ArrowLeftRight,
  CalendarDays,
  CalendarOff,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Church,
  Plus,
  RefreshCw,
  Send,
  Sparkles,
  Trash2,
  TriangleAlert,
  X,
} from "lucide-react"
import { AnimatePresence, motion, useReducedMotion } from "motion/react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useSearchParams } from "react-router"
import { toast } from "sonner"

import { CommunionTag } from "@/components/communion-tag"
import { ConfirmButton } from "@/components/confirm-button"
import { DateBlock } from "@/components/date-block"
import { EmptyState } from "@/components/empty-state"
import { ErrorState } from "@/components/error-state"
import { CardSkeleton, ListSkeleton } from "@/components/list-skeleton"
import { quick, Reveal, RevealItem, staggerDelay } from "@/components/motion"
import { PageHeader } from "@/components/page-header"
import { RoleIcon } from "@/components/role-icon"
import { SlotStatusIcon } from "@/components/slot-status"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import {
  ApiError,
  api,
  queryKeys,
  type CoverRequest,
  type MinistryDetail,
  type ScheduleRole,
  type ScheduleService,
  type ScheduleSlot,
  type ScheduleView,
  type Unavailability,
} from "@/lib/api"
import { toastError } from "@/lib/errors"
import {
  absenceLabel,
  dayAndTime,
  firstName,
  formatDay,
  initials,
  monthBounds,
  monthLabel,
  nextMonth,
  reasonLabel,
  shiftMonth,
  todayISO,
  weekRange,
  weekdayOf,
  weekdayShort,
} from "@/lib/format"
import { cn } from "@/lib/utils"

import { SlotDialog, type SlotSelection } from "./slot-dialog"
import { UnavailabilityDialog } from "./unavailability-dialog"

export function SchedulesPage() {
  const { t } = useTranslation()
  const ministries = useQuery({ queryKey: queryKeys.ministries, queryFn: api.ministries.list })
  const [params, setParams] = useSearchParams()
  const [direction, setDirection] = useState(0)
  const month = params.get("mes") ?? nextMonth()
  const ministryId = Number(params.get("ministerio")) || ministries.data?.[0]?.id

  const update = (changes: Record<string, string>) => setParams({ ...Object.fromEntries(params), ...changes }, { replace: true })
  const go = (delta: number) => {
    setDirection(delta)
    update({ mes: shiftMonth(month, delta) })
  }

  if (ministries.isPending) {
    return (
      <>
        <PageHeader title={t("schedules.title")} />
        <BoardSkeleton />
      </>
    )
  }
  if (ministries.isError) {
    return (
      <>
        <PageHeader title={t("schedules.title")} />
        <ErrorState error={ministries.error} onRetry={() => void ministries.refetch()} retrying={ministries.isFetching} />
      </>
    )
  }
  if (!ministryId) {
    return (
      <>
        <PageHeader title={t("schedules.title")} />
        <EmptyState
          icon={Church}
          title={t("schedules.noMinistryTitle")}
          action={
            <Button asChild>
              <Link to="/ministerios">{t("schedules.goToMinistries")}</Link>
            </Button>
          }
        >
          {t("schedules.noMinistry")}
        </EmptyState>
      </>
    )
  }

  return (
    <>
      <PageHeader
        title={t("schedules.title")}
        action={
          ministries.data.length > 1 && (
            <Select value={String(ministryId)} onValueChange={(value) => update({ ministerio: value })}>
              <SelectTrigger className="w-48 max-w-[55vw]" aria-label={t("schedules.ministry")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ministries.data.map((m) => (
                  <SelectItem key={m.id} value={String(m.id)}>
                    {m.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )
        }
      />
      <MonthPager month={month} direction={direction} onGo={go} />
      <ScheduleBoard key={`${ministryId}-${month}`} ministryId={ministryId} month={month} direction={direction} />
    </>
  )
}

function MonthPager({ month, direction, onGo }: { month: string; direction: number; onGo: (delta: number) => void }) {
  const { t } = useTranslation()
  return (
    <div className="mb-6 flex items-center gap-2 rounded-2xl border bg-card p-1.5">
      <Button variant="ghost" size="icon" aria-label={t("schedules.previousMonth")} onClick={() => onGo(-1)}>
        <ChevronLeft />
      </Button>
      <div className="relative flex h-10 flex-1 items-center justify-center overflow-hidden" aria-live="polite">
        <AnimatePresence mode="popLayout" initial={false} custom={direction}>
          <motion.span
            key={month}
            custom={direction}
            initial={{ opacity: 0, x: direction * 28 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: direction * -28 }}
            transition={quick}
            className="font-display text-lg font-semibold"
          >
            {monthLabel(month)}
          </motion.span>
        </AnimatePresence>
      </div>
      <Button variant="ghost" size="icon" aria-label={t("schedules.nextMonth")} onClick={() => onGo(1)}>
        <ChevronRight />
      </Button>
    </div>
  )
}

function BoardSkeleton() {
  return (
    <div className="flex flex-col gap-4">
      <Skeleton className="h-6 w-40 rounded-full" />
      <CardSkeleton lines={5} />
      <CardSkeleton lines={5} />
    </div>
  )
}

function ScheduleBoard({ ministryId, month, direction }: { ministryId: number; month: string; direction: number }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const ministry = useQuery({ queryKey: queryKeys.ministry(ministryId), queryFn: () => api.ministries.get(ministryId) })
  const schedule = useQuery({
    queryKey: queryKeys.schedule(ministryId, month),
    queryFn: () => api.schedules.get(ministryId, month),
  })
  const range = scheduleRange(schedule.data) ?? monthBounds(month)
  const unavailabilities = useQuery({
    queryKey: queryKeys.unavailabilities(range.from, range.to),
    queryFn: () => api.unavailabilities.list(range.from, range.to),
  })
  const [selection, setSelection] = useState<SlotSelection | undefined>()

  const generate = useMutation({
    mutationFn: () => api.schedules.generate(ministryId, month),
    onSuccess: (view) => {
      queryClient.setQueryData(queryKeys.schedule(ministryId, month), view)
      toast.success(t("schedules.generated"))
    },
    onError: toastError,
  })

  const missing = schedule.error instanceof ApiError && schedule.error.status === 404
  const members = new Set(ministry.data?.members.map((member) => member.personId))
  const absences = (unavailabilities.data ?? []).filter((u) => members.has(u.personId))

  return (
    <motion.div
      initial={{ opacity: 0, x: direction * 24 }}
      animate={{ opacity: 1, x: 0 }}
      transition={quick}
      className="flex flex-col gap-8"
    >
      {schedule.isPending || ministry.isPending ? (
        <BoardSkeleton />
      ) : missing ? (
        <EmptyMonth ministry={ministry.data} month={month} generating={generate.isPending} onGenerate={() => generate.mutate()} />
      ) : schedule.isError || ministry.isError ? (
        <ErrorState
          error={schedule.error ?? ministry.error}
          onRetry={() => {
            void schedule.refetch()
            void ministry.refetch()
          }}
          retrying={schedule.isFetching || ministry.isFetching}
        />
      ) : (
        <Board view={schedule.data} ministry={ministry.data} month={month} onSelect={setSelection} onRegenerate={() => generate.mutate()} regenerating={generate.isPending} />
      )}

      {schedule.data?.status === "published" && <CoversCard ministryId={ministryId} month={month} />}

      <AbsencesCard
        list={absences}
        loading={unavailabilities.isPending || ministry.isPending}
        error={unavailabilities.isError ? unavailabilities.error : null}
        onRetry={() => void unavailabilities.refetch()}
        defaultDate={range.from}
      />

      {ministry.data && (
        <SlotDialog
          selection={selection}
          onClose={() => setSelection(undefined)}
          ministry={ministry.data}
          month={month}
          published={schedule.data?.status === "published"}
          unavailabilities={unavailabilities.data ?? []}
        />
      )}
    </motion.div>
  )
}

function EmptyMonth({
  ministry,
  month,
  generating,
  onGenerate,
}: {
  ministry?: MinistryDetail
  month: string
  generating: boolean
  onGenerate: () => void
}) {
  const { t } = useTranslation()
  const ready = Boolean(ministry && ministry.roles.length > 0 && ministry.members.length > 0 && ministry.serviceIds.length > 0)
  return (
    <EmptyState
      icon={CalendarDays}
      title={t("schedules.emptyMonthTitle", { month: monthLabel(month).toLowerCase() })}
      action={
        ready ? (
          <Button onClick={onGenerate} disabled={generating}>
            <Sparkles />
            {generating ? t("schedules.generating") : t("schedules.generate")}
          </Button>
        ) : (
          ministry && (
            <Button variant="outline" asChild>
              <Link to={`/ministerios/${ministry.id}`}>{t("schedules.goToMinistries")}</Link>
            </Button>
          )
        )
      }
    >
      {ready ? t("schedules.emptyMonth") : t("schedules.notReady")}
    </EmptyState>
  )
}

const WARNING_KEYS = {
  unavailable: "schedules.warning.unavailable",
  doubleBooked: "schedules.warning.doubleBooked",
  notMember: "schedules.warning.notMember",
} as const

type Flagged = { service: ScheduleService; role: ScheduleRole; slot: ScheduleSlot }

function flagged(view: ScheduleView): Flagged[] {
  return view.weeks.flatMap((week) =>
    week.services.flatMap((service) =>
      service.roles.flatMap((role) => role.slots.filter((slot) => slot.warning).map((slot) => ({ service, role, slot })))
    )
  )
}

function Board({
  view,
  ministry,
  month,
  onSelect,
  onRegenerate,
  regenerating,
}: {
  view: ScheduleView
  ministry: MinistryDetail
  month: string
  onSelect: (selection: SlotSelection) => void
  onRegenerate: () => void
  regenerating: boolean
}) {
  const { t } = useTranslation()
  const today = todayISO()
  const published = view.status === "published"

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center gap-2">
        {published ? (
          <Badge variant="success">
            <Check />
            {t("schedules.published")}
          </Badge>
        ) : (
          <Badge variant="secondary">{t("schedules.draft")}</Badge>
        )}
        {published && view.warnings > 0 && <WarningJump count={view.warnings} />}
      </div>

      {view.weeks.map((week, index) => (
        <Reveal key={week.start} delay={staggerDelay(index)}>
          <section aria-labelledby={`week-${week.start}`} className="overflow-hidden rounded-2xl border bg-card">
            <h2
              id={`week-${week.start}`}
              className="border-b bg-muted/30 px-4 py-2.5 text-xs font-semibold tracking-wide text-muted-foreground uppercase"
            >
              {t("schedules.week", { range: weekRange(week.start) })}
            </h2>
            <div className="divide-y">
              {week.services.map((service) => (
                <ServiceRow
                  key={`${service.date}-${service.serviceId}`}
                  service={service}
                  past={service.date < today}
                  published={published}
                  onSelect={onSelect}
                />
              ))}
            </div>
          </section>
        </Reveal>
      ))}

      {!published && <DraftBar view={view} ministry={ministry} month={month} onRegenerate={onRegenerate} regenerating={regenerating} />}
    </div>
  )
}

function ServiceRow({
  service,
  past,
  published,
  onSelect,
}: {
  service: ScheduleService
  past: boolean
  published: boolean
  onSelect: (selection: SlotSelection) => void
}) {
  const { t } = useTranslation()
  return (
    <article className="px-4 py-4">
      <div className={cn("flex items-start gap-3", past && "opacity-60")}>
        <DateBlock date={service.date} startTime={service.startTime} />
        <div className="min-w-0 flex-1 pt-1">
          <h3 className="font-medium text-pretty">
            {service.name}
            <span className="sr-only">, {dayAndTime(service.date, service.startTime)}</span>
          </h3>
          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1">
            {service.communion && <CommunionTag />}
            {past && <span className="text-xs text-muted-foreground">{t("schedules.past")}</span>}
          </div>
        </div>
      </div>
      <dl className="mt-3 grid gap-2">
        {service.roles.map((role) => {
          return (
            <div key={role.roleId} className="grid grid-cols-[minmax(0,6.75rem)_minmax(0,1fr)] items-start gap-x-3 sm:grid-cols-[12rem_minmax(0,1fr)]">
              <dt className="flex min-w-0 items-center gap-1.5 pt-1.5 text-sm text-muted-foreground">
                <RoleIcon name={role.name} className="size-4 shrink-0" aria-hidden />
                <span className="truncate" title={role.name}>
                  {role.name}
                </span>
              </dt>
              <dd className="flex min-w-0 flex-wrap gap-1.5">
                {role.slots.map((slot) => (
                  <SlotChip
                    key={slot.assignmentId}
                    slot={slot}
                    role={role.name}
                    published={published}
                    onClick={() => onSelect({ service, role, slot })}
                  />
                ))}
              </dd>
            </div>
          )
        })}
      </dl>
    </article>
  )
}

function SlotChip({ slot, role, published, onClick }: { slot: ScheduleSlot; role: string; published: boolean; onClick: () => void }) {
  const { t } = useTranslation()
  const empty = slot.warning === "empty" || slot.personId === 0
  const warning = slot.warning && slot.warning !== "empty" ? t(WARNING_KEYS[slot.warning]) : null
  const name = empty ? t("schedules.vacant") : slot.personName
  return (
    <button
      type="button"
      onClick={onClick}
      data-warning={slot.warning ? "" : undefined}
      aria-label={[t("schedules.slotLabel", { role, name }), warning].filter(Boolean).join(", ")}
      className={cn(
        "inline-flex h-8 max-w-full min-w-0 items-center gap-1.5 rounded-full border px-3 text-sm transition-[background-color,border-color,box-shadow] duration-150 hover:bg-accent focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none active:scale-[0.97]",
        empty
          ? "border-dashed border-danger/50 text-danger"
          : slot.warning
            ? "border-danger/40 bg-danger/10 text-danger"
            : "border-border bg-secondary text-secondary-foreground"
      )}
    >
      {published && !empty && <SlotStatusIcon status={slot.status} coverOpen={slot.coverOpen} />}
      <span className={cn("truncate", slot.status === "declined" && !slot.coverOpen && "text-muted-foreground")}>{name}</span>
      {warning && (
        <>
          <TriangleAlert className="size-3.5 shrink-0 sm:hidden" aria-hidden />
          <span className="hidden shrink-0 text-xs opacity-90 sm:inline" aria-hidden>
            · {warning}
          </span>
        </>
      )}
    </button>
  )
}

/**
 * NOTE: Scrolls to the next slot that needs a look, wrapping to the first, and flashes it so the eye lands on it.
 */
function useWarningJump() {
  const reduce = useReducedMotion()
  return () => {
    const slots = [...document.querySelectorAll<HTMLElement>("[data-warning]")]
    if (slots.length === 0) {
      return
    }
    const next = slots.find((el) => el.getBoundingClientRect().top > window.innerHeight * 0.55) ?? slots[0]
    next.scrollIntoView({ behavior: reduce ? "auto" : "smooth", block: "center" })
    next.classList.remove("slot-flash")
    void next.offsetWidth
    next.classList.add("slot-flash")
  }
}

function WarningJump({ count }: { count: number }) {
  const { t } = useTranslation()
  const jump = useWarningJump()
  return (
    <button
      type="button"
      onClick={jump}
      aria-label={t("schedules.nextWarning")}
      className="inline-flex h-8 items-center gap-1.5 rounded-full bg-warning/12 px-3 text-xs font-medium text-warning transition-colors hover:bg-warning/20"
    >
      <TriangleAlert className="size-3.5" />
      {t("schedules.warnings", { count })}
      <ChevronDown className="size-3.5" />
    </button>
  )
}

/**
 * NOTE: The draft's actions stay under the thumb while the leader scrolls through the month, and stop at the end of
 * the board.
 */
function DraftBar({
  view,
  ministry,
  month,
  onRegenerate,
  regenerating,
}: {
  view: ScheduleView
  ministry: MinistryDetail
  month: string
  onRegenerate: () => void
  regenerating: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const jump = useWarningJump()
  const pending = flagged(view)

  const discard = useMutation({
    mutationFn: () => api.schedules.discard(ministry.id, month),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: queryKeys.schedule(ministry.id, month) })
      queryClient.invalidateQueries({ queryKey: queryKeys.schedule(ministry.id, month) })
      toast.success(t("schedules.discarded"))
    },
    onError: toastError,
  })
  const publish = useMutation({
    mutationFn: () => api.schedules.publish(ministry.id, month),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.schedule(ministry.id, month) })
      toast.success(ministry.whatsappGroup ? t("schedules.publishedToastGroup") : t("schedules.publishedToast"))
    },
    onError: toastError,
  })

  return (
    <div className="sticky bottom-[calc(4.25rem+env(safe-area-inset-bottom)+0.75rem)] z-10 lg:bottom-6">
      <div className="flex items-center gap-1.5 rounded-2xl border bg-popover/95 p-2 pl-3 shadow-[0_16px_40px_oklch(0_0_0/0.45)] backdrop-blur-md">
        {pending.length > 0 ? (
          <button
            type="button"
            onClick={jump}
            aria-label={t("schedules.nextWarning")}
            className="-my-1 flex min-h-11 min-w-0 flex-1 items-center gap-1.5 rounded-lg text-left text-sm font-medium text-warning"
          >
            <TriangleAlert className="size-4 shrink-0" />
            <span className="truncate sm:hidden">{t("schedules.warningsShort", { count: pending.length })}</span>
            <span className="hidden truncate sm:inline">{t("schedules.warnings", { count: pending.length })}</span>
            <ChevronDown className="size-4 shrink-0" />
          </button>
        ) : (
          <span className="flex min-w-0 flex-1 items-center gap-1.5 text-sm font-medium text-success">
            <Check className="size-4 shrink-0" />
            <span className="truncate">{t("schedules.allFilled")}</span>
          </span>
        )}
        <ConfirmButton
          tone="outline"
          className="max-sm:w-10 max-sm:px-0"
          icon={<Trash2 />}
          label={<span className="sr-only sm:not-sr-only">{t("schedules.discard")}</span>}
          aria-label={t("schedules.discard")}
          confirmLabel={t("schedules.discard")}
          title={t("schedules.discardTitle")}
          description={t("schedules.discardLead")}
          onConfirm={() => discard.mutate()}
          pending={discard.isPending}
        />
        <ConfirmButton
          tone="outline"
          className="max-sm:w-10 max-sm:px-0"
          icon={<RefreshCw />}
          label={<span className="sr-only sm:not-sr-only">{t("schedules.regenerate")}</span>}
          aria-label={t("schedules.regenerate")}
          confirmLabel={t("schedules.regenerate")}
          title={t("schedules.regenerateTitle")}
          description={t("schedules.regenerateLead")}
          onConfirm={onRegenerate}
          pending={regenerating}
        />
        <ConfirmButton
          tone="primary"
          icon={<Send />}
          label={t("schedules.publish")}
          confirmLabel={pending.length > 0 ? t("schedules.publishAnyway") : t("schedules.publish")}
          title={t("schedules.publishTitle", { month: monthLabel(month).toLowerCase() })}
          description={`${t("schedules.publishLead")}${ministry.whatsappGroup ? ` ${t("schedules.publishGroup")}` : ""}`}
          onConfirm={() => publish.mutate()}
          pending={publish.isPending}
        >
          {pending.length > 0 && (
            <div className="flex flex-col gap-2 rounded-xl bg-warning/10 p-3">
              <p className="flex items-center gap-1.5 text-sm font-medium text-warning">
                <TriangleAlert className="size-4" />
                {t("schedules.publishPending")}
              </p>
              <ul className="flex flex-col gap-1 text-sm">
                {pending.map(({ service, role, slot }) => (
                  <li key={slot.assignmentId} className="flex min-w-0 gap-2">
                    <span className="tabular shrink-0 text-muted-foreground">
                      {weekdayShort(service.date)} {formatDay(service.date)}
                    </span>
                    <span className="truncate">
                      {role.name} · {slot.personId === 0 ? t("schedules.vacant") : `${firstName(slot.personName)}, ${slot.warning && slot.warning !== "empty" ? t(WARNING_KEYS[slot.warning]) : ""}`}
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </ConfirmButton>
      </div>
    </div>
  )
}

function AbsencesCard({
  list,
  loading,
  error,
  onRetry,
  defaultDate,
}: {
  list: Unavailability[]
  loading: boolean
  error: Error | null
  onRetry: () => void
  defaultDate: string
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [swapsFor, setSwapsFor] = useState<Unavailability>()
  const remove = useMutation({
    mutationFn: (id: number) => api.unavailabilities.remove(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["unavailabilities"] })
      queryClient.invalidateQueries({ queryKey: ["schedules"] })
      toast.success(t("schedules.absenceRemoved"))
    },
    onError: toastError,
  })

  return (
    <section aria-labelledby="absences" className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3">
        <h2 id="absences" className="flex items-center gap-2 text-lg font-semibold">
          <CalendarOff className="size-5 text-brand" />
          {t("schedules.absences")}
        </h2>
        <Button size="sm" variant="outline" onClick={() => setOpen(true)}>
          <Plus />
          {t("schedules.addAbsence")}
        </Button>
      </div>
      {loading ? (
        <ListSkeleton rows={2} />
      ) : error ? (
        <ErrorState error={error} onRetry={onRetry} />
      ) : list.length === 0 ? (
        <p className="rounded-2xl border border-dashed px-4 py-5 text-sm text-pretty text-muted-foreground">{t("schedules.absencesEmpty")}</p>
      ) : (
        <ul className="divide-y rounded-2xl border bg-card">
          {list.map((u, index) => (
            <RevealItem key={u.id} index={index} className="flex flex-col gap-2 px-4 py-3">
              <div className="flex items-center gap-3">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-secondary text-xs font-semibold text-secondary-foreground">
                  {initials(u.personName)}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium">{u.personName}</p>
                  <p className="text-xs text-pretty text-muted-foreground">
                    {absenceLabel(u)} · {u.fromPoll ? t("schedules.fromPoll") : reasonLabel(u.reason)}
                  </p>
                </div>
                <ConfirmButton
                  size="icon-sm"
                  icon={<X />}
                  label={null}
                  aria-label={t("schedules.removeAbsenceLabel", { name: u.personName })}
                  confirmLabel={t("schedules.removeAbsence")}
                  title={t("schedules.removeAbsenceTitle", { name: firstName(u.personName) })}
                  description={t("schedules.removeAbsenceLead", { label: absenceLabel(u) })}
                  onConfirm={() => remove.mutate(u.id)}
                  pending={remove.isPending && remove.variables === u.id}
                  className="text-muted-foreground hover:text-danger"
                />
              </div>
              {u.scheduled > 0 && (
                <Button size="sm" variant="outline" className="self-start sm:ml-12" onClick={() => setSwapsFor(u)}>
                  <ArrowLeftRight />
                  {u.scheduled === 1 ? t("schedules.findCover") : t("schedules.findCovers", { count: u.scheduled })}
                </Button>
              )}
            </RevealItem>
          ))}
        </ul>
      )}
      <UnavailabilityDialog open={open} onOpenChange={setOpen} defaultDate={defaultDate} />
      <UnavailabilityDialog
        open={Boolean(swapsFor)}
        onOpenChange={(next) => !next && setSwapsFor(undefined)}
        defaultDate={defaultDate}
        absence={swapsFor}
      />
    </section>
  )
}

function scheduleRange(view?: ScheduleView): { from: string; to: string } | undefined {
  const dates = view?.weeks.flatMap((week) => week.services.map((service) => service.date)) ?? []
  if (dates.length === 0) {
    return undefined
  }
  return { from: dates[0], to: dates[dates.length - 1] }
}

const COVER_BADGE = { open: "warning", covered: "success", uncovered: "destructive" } as const

function CoversCard({ ministryId, month }: { ministryId: number; month: string }) {
  const { t } = useTranslation()
  const covers = useQuery({
    queryKey: queryKeys.covers(ministryId, month),
    queryFn: () => api.schedules.covers(ministryId, month),
    refetchInterval: 30_000,
    meta: { background: true },
  })
  if (!covers.data || covers.data.length === 0) {
    return null
  }
  return (
    <section aria-labelledby="covers" className="flex flex-col gap-3">
      <h2 id="covers" className="flex items-center gap-2 text-lg font-semibold">
        <ArrowLeftRight className="size-5 text-brand" />
        {t("schedules.covers")}
      </h2>
      <ul className="divide-y rounded-2xl border bg-card">
        {covers.data.map((cover, index) => (
          <CoverItem key={cover.id} cover={cover} index={index} />
        ))}
      </ul>
    </section>
  )
}

function CoverItem({ cover, index }: { cover: CoverRequest; index: number }) {
  const { t } = useTranslation()
  const answers = cover.invites.map((invite) =>
    invite.answer === "no"
      ? t("schedules.inviteNo", { name: firstName(invite.name) })
      : invite.answer === "yes"
        ? t("schedules.inviteYes", { name: firstName(invite.name) })
        : firstName(invite.name)
  )
  return (
    <RevealItem index={index} className="flex flex-col gap-1.5 px-4 py-3">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-sm font-medium">
          {weekdayOf(cover.date)}, {formatDay(cover.date)} · {cover.roleName}
        </span>
        <Badge variant={COVER_BADGE[cover.status]}>{t(`schedules.coverStatus.${cover.status}`)}</Badge>
        {cover.late && <Badge variant="outline">{t("schedules.lateNotice")}</Badge>}
      </div>
      <p className="text-xs text-pretty text-muted-foreground">
        {t("schedules.coverAbsent", { name: cover.absentName })}
        {cover.status === "covered" && ` ${t("schedules.coverCovered", { name: cover.coveredBy })}`}
        {cover.status !== "covered" && answers.length > 0 && ` ${t("schedules.coverInvited", { names: answers.join(", ") })}`}
        {cover.status !== "covered" && answers.length === 0 && ` ${t("schedules.coverNobody")}`}
      </p>
    </RevealItem>
  )
}

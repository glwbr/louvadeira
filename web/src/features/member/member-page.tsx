import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { CalendarDays, CalendarOff, Check, Hourglass, Loader2, Plus } from "lucide-react"
import { AnimatePresence, motion } from "motion/react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { CommunionTag } from "@/components/communion-tag"
import { ConfirmButton } from "@/components/confirm-button"
import { DateBlock } from "@/components/date-block"
import { EmptyState } from "@/components/empty-state"
import { ErrorState } from "@/components/error-state"
import { IconTile } from "@/components/icon-tile"
import { ListCard } from "@/components/list"
import { QueryState } from "@/components/query-state"
import { SectionHeader } from "@/components/section-header"
import { CardSkeleton, ListSkeleton } from "@/components/list-skeleton"
import { AnimatedItem, AnimatedList, EASE_OUT, quick, spring } from "@/components/motion"
import { RoleIcon } from "@/components/role-icon"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { api, queryKeys, type AssignmentStatus, type MemberSlot, type Unavailability } from "@/lib/api"
import { queries } from "@/lib/queries"
import { toastError } from "@/lib/errors"
import { absenceLabel, firstName, formatDay, reasonLabel, weekdayOf } from "@/lib/format"
import { cn } from "@/lib/utils"

import { AbsenceRow } from "@/features/schedules/absence-row"

import { MyAbsenceDialog } from "./my-absence-dialog"

type Service = {
  key: string
  date: string
  startTime: string
  serviceName: string
  ministryName: string
  communion: boolean
  roles: string[]
  slots: MemberSlot[]
}

function groupByService(slots: MemberSlot[]): Service[] {
  const services: Service[] = []
  for (const slot of slots) {
    const key = `${slot.date}#${slot.startTime}#${slot.ministryId}`
    const last = services.at(-1)
    if (last?.key === key) {
      last.roles.push(slot.roleName)
      last.slots.push(slot)
      continue
    }
    services.push({
      key,
      date: slot.date,
      startTime: slot.startTime,
      serviceName: slot.serviceName,
      ministryName: slot.ministryName,
      communion: slot.communion,
      roles: [slot.roleName],
      slots: [slot],
    })
  }
  return services
}

export function MemberPage() {
  const { t } = useTranslation()
  const me = useQuery(queries.me())
  const slots = useQuery({ queryKey: queryKeys.mySlots, queryFn: api.mine.slots })
  const absences = useQuery({ queryKey: queryKeys.myAbsences, queryFn: api.mine.absences })

  return (
    <div className="flex flex-col gap-10">
      <div>
        <h1 className="font-display text-[2rem] leading-[1.1] font-semibold text-balance">
          {me.data ? t("member.greeting", { name: firstName(me.data.name) }) : <Skeleton className="h-9 w-44" />}
        </h1>
        <p className="mt-2 max-w-prose text-sm text-pretty text-muted-foreground">{t("member.lead")}</p>
      </div>

      <section aria-labelledby="my-services" className="flex flex-col gap-3">
        <h2 id="my-services" className="text-lg font-semibold">
          {t("member.upcoming")}
        </h2>
        <QueryState
          query={slots}
          loading={
            <div className="flex flex-col gap-3">
              <CardSkeleton />
              <CardSkeleton />
            </div>
          }
          isEmpty={(data) => data.length === 0}
          empty={
            <EmptyState icon={CalendarDays} title={t("member.emptyTitle")}>
              {t("member.empty")}
            </EmptyState>
          }
        >
          {(data) => (
            <ul className="flex flex-col gap-3">
              <AnimatedList>
                {groupByService(data).map((service) => (
                  <AnimatedItem key={service.key}>
                    <ServiceCard service={service} />
                  </AnimatedItem>
                ))}
              </AnimatedList>
            </ul>
          )}
        </QueryState>
      </section>

      <AbsencesSection
        absences={absences.data}
        loading={absences.isPending}
        error={absences.isError ? absences.error : null}
        onRetry={() => void absences.refetch()}
      />
    </div>
  )
}

function masculineDay(date: string): boolean {
  const day = new Date(`${date}T12:00:00`).getDay()
  return day === 0 || day === 6
}

function ServiceCard({ service }: { service: Service }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const ids = service.slots.map((slot) => slot.assignmentId)
  const declined = service.slots.every((slot) => slot.status === "declined")
  const confirmed = service.slots.every((slot) => slot.status === "confirmed")
  const searching = service.slots.some((slot) => slot.coverOpen)
  const day = `${weekdayOf(service.date).toLowerCase()}, ${formatDay(service.date)}`

  /**
   * NOTE: The answer shows on the card at once and the server catches up behind it; if it fails, the card goes back
   * to how it was and the error says why.
   */
  const answer = (status: AssignmentStatus) => async () => {
    await queryClient.cancelQueries({ queryKey: queryKeys.mySlots })
    const previous = queryClient.getQueryData<MemberSlot[]>(queryKeys.mySlots)
    queryClient.setQueryData<MemberSlot[]>(queryKeys.mySlots, (slots) =>
      slots?.map((slot) => (ids.includes(slot.assignmentId) ? { ...slot, status } : slot))
    )
    return { previous }
  }
  const rollback = (error: Error, _: unknown, context?: { previous?: MemberSlot[] }) => {
    queryClient.setQueryData(queryKeys.mySlots, context?.previous)
    toastError(error)
  }
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["mine"] })
  const confirm = useMutation({
    mutationFn: () => api.mine.confirm(ids),
    onMutate: answer("confirmed"),
    onSuccess: () => toast.success(declined ? t("member.back") : t("member.confirmed")),
    onError: rollback,
    onSettled: refresh,
  })
  const decline = useMutation({
    mutationFn: () => api.mine.decline(ids.filter((_, i) => service.slots[i].status !== "declined")),
    onMutate: answer("declined"),
    onSuccess: (requests) => {
      const invited = requests.some((request) => request.invites.length > 0)
      toast.success(invited ? t("member.declinedSearching") : t("member.declinedNobody"))
    },
    onError: rollback,
    onSettled: refresh,
  })

  return (
    <article
      className={cn(
        "rounded-2xl border bg-card p-4 transition-[border-color,box-shadow] duration-500",
        confirmed && "border-success/35 shadow-[0_0_0_1px_color-mix(in_oklch,var(--success)_12%,transparent)]"
      )}
    >
      <div className="flex gap-4">
        <DateBlock date={service.date} startTime={service.startTime} />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-1.5">
            <h3 className="min-w-0 font-medium text-pretty">
              {service.serviceName}
              <span className="sr-only">, {day}</span>
            </h3>
            <Status confirmed={confirmed} declined={declined} searching={searching} />
          </div>
          <p className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground">
            <span>{service.ministryName}</span>
            {service.roles.map((role) => (
              <span key={role} className="inline-flex items-center gap-1">
                <RoleIcon name={role} className="size-3.5" aria-hidden />
                {role}
              </span>
            ))}
          </p>
          {service.communion && <CommunionTag className="mt-1.5" />}
        </div>
      </div>

      <AnimatePresence initial={false}>
        {!confirmed && (
          <motion.div
            key="actions"
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0, transition: { duration: 0.25, ease: EASE_OUT } }}
            className="overflow-hidden"
          >
            <div className="flex flex-wrap gap-2 pt-4">
              {declined ? (
                <Button variant="outline" className="flex-1 sm:flex-none" disabled={confirm.isPending} onClick={() => confirm.mutate()}>
                  {confirm.isPending && <Loader2 className="animate-spin" />}
                  {t("member.comeBack")}
                </Button>
              ) : (
                <>
                  <Button
                    className="flex-1 sm:flex-none"
                    disabled={confirm.isPending}
                    aria-label={t("member.confirmLabel", { day })}
                    onClick={() => confirm.mutate()}
                  >
                    {confirm.isPending ? <Loader2 className="animate-spin" /> : <Check />}
                    {t("member.confirm")}
                  </Button>
                  <ConfirmButton
                    tone="outline"
                    className="flex-1 sm:flex-none"
                    label={t("member.decline")}
                    aria-label={t("member.declineLabel", { day })}
                    title={t(masculineDay(service.date) ? "member.declineTitleMasculine" : "member.declineTitleFeminine", { day })}
                    description={t("member.declineLead")}
                    onConfirm={() => decline.mutate()}
                    pending={decline.isPending}
                  />
                </>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </article>
  )
}

function Status({ confirmed, declined, searching }: { confirmed: boolean; declined: boolean; searching: boolean }) {
  const { t } = useTranslation()
  const state = declined ? (searching ? "searching" : "leadersKnow") : confirmed ? "confirmed" : "pending"
  return (
    <AnimatePresence mode="popLayout" initial={false}>
      <motion.span
        key={state}
        initial={{ opacity: 0, scale: 0.85 }}
        animate={{ opacity: 1, scale: 1 }}
        exit={{ opacity: 0, scale: 0.85 }}
        transition={spring}
        className={cn(
          "inline-flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium",
          state === "confirmed" && "bg-success/12 text-success",
          state === "searching" && "bg-warning/12 text-warning",
          state === "leadersKnow" && "bg-secondary text-muted-foreground",
          state === "pending" && "bg-secondary text-secondary-foreground"
        )}
      >
        {state === "confirmed" ? (
          <DrawnCheck />
        ) : state === "searching" ? (
          <Hourglass className="size-3.5" />
        ) : state === "leadersKnow" ? (
          <CalendarOff className="size-3.5" />
        ) : (
          <span className="size-1.5 rounded-full bg-brand" />
        )}
        {t(`member.status.${state}`)}
      </motion.span>
    </AnimatePresence>
  )
}

/**
 * NOTE: The one authored moment of the member page: confirming draws the check instead of swapping an icon.
 */
function DrawnCheck() {
  return (
    <svg viewBox="0 0 24 24" className="size-3.5" fill="none" stroke="currentColor" strokeWidth={3} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <motion.path d="M5 12.5l4.5 4.5L19 7.5" initial={{ pathLength: 0 }} animate={{ pathLength: 1 }} transition={{ ...quick, duration: 0.45, delay: 0.1 }} />
    </svg>
  )
}

function AbsencesSection({
  absences,
  loading,
  error,
  onRetry,
}: {
  absences?: Unavailability[]
  loading: boolean
  error: Error | null
  onRetry: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [adding, setAdding] = useState(false)
  const [coversFor, setCoversFor] = useState<Unavailability>()
  const remove = useMutation({
    mutationFn: (id: number) => api.mine.removeAbsence(id),
    onSuccess: () => {
      toast.success(t("member.removed"))
      queryClient.invalidateQueries({ queryKey: ["mine"] })
    },
    onError: toastError,
  })

  return (
    <section aria-labelledby="my-absences" className="flex flex-col gap-3">
      <SectionHeader
        id="my-absences"
        title={t("member.absences")}
        action={
          <Button size="sm" variant="outline" onClick={() => setAdding(true)}>
            <Plus />
            {t("member.addAbsence")}
          </Button>
        }
      />
      {loading ? (
        <ListSkeleton rows={1} />
      ) : error ? (
        <ErrorState error={error} onRetry={onRetry} />
      ) : !absences?.length ? (
        <p className="rounded-2xl border border-dashed px-4 py-5 text-sm text-pretty text-muted-foreground">{t("member.absencesEmpty")}</p>
      ) : (
        <ListCard>
          {absences.map((u) => (
            <AbsenceRow
              key={u.id}
              leading={<IconTile icon={CalendarOff} className="size-9 [&>svg]:size-4" />}
              title={absenceLabel(u)}
              subtitle={u.fromPoll ? t("member.fromPoll") : reasonLabel(u.reason)}
              cover={u.scheduled > 0 ? { label: t("member.findCover"), onClick: () => setCoversFor(u) } : undefined}
              remove={{
                label: t("member.removeLabel", { label: absenceLabel(u) }),
                confirmLabel: t("member.remove"),
                title: t("member.removeTitle"),
                description: t("member.removeLead", { label: absenceLabel(u) }),
                onConfirm: () => remove.mutate(u.id),
                pending: remove.isPending && remove.variables === u.id,
              }}
            />
          ))}
        </ListCard>
      )}
      <MyAbsenceDialog open={adding} onOpenChange={setAdding} />
      <MyAbsenceDialog open={Boolean(coversFor)} onOpenChange={(next) => !next && setCoversFor(undefined)} absence={coversFor} />
    </section>
  )
}

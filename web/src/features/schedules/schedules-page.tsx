import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { CalendarDays, CalendarOff, ChevronLeft, ChevronRight, Church, Plus, RefreshCw, Send, Sparkles, Trash2, X } from "lucide-react"
import { useState } from "react"
import { Link, useSearchParams } from "react-router"
import { toast } from "sonner"

import { ConfirmButton } from "@/components/confirm-button"
import { EmptyState } from "@/components/empty-state"
import { ListSkeleton } from "@/components/list-skeleton"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { ApiError, api, queryKeys, type ScheduleSlot, type ScheduleView, type Unavailability } from "@/lib/api"
import { toastError } from "@/lib/errors"
import {
  REASON_LABEL,
  formatDateRange,
  formatDay,
  monthBounds,
  monthLabel,
  nextMonth,
  shiftMonth,
  weekRange,
  weekdayOf,
} from "@/lib/format"
import { cn } from "@/lib/utils"
import { SlotDialog, type SlotSelection } from "./slot-dialog"
import { UnavailabilityDialog } from "./unavailability-dialog"

export function SchedulesPage() {
  const ministries = useQuery({ queryKey: queryKeys.ministries, queryFn: api.ministries.list })
  const [params, setParams] = useSearchParams()
  const month = params.get("mes") ?? nextMonth()
  const ministryId = Number(params.get("ministerio")) || ministries.data?.[0]?.id

  const update = (changes: Record<string, string>) => setParams({ ...Object.fromEntries(params), ...changes }, { replace: true })

  if (ministries.isPending) {
    return <ListSkeleton rows={4} />
  }
  if (!ministryId) {
    return (
      <>
        <PageHeader title="Escalas" />
        <EmptyState icon={Church} title="Nenhum ministério ainda">
          Crie o ministério em{" "}
          <Link to="/ministerios" className="text-primary underline">
            Ministérios
          </Link>{" "}
          com funções, equipe e cultos. Depois volte para gerar a escala.
        </EmptyState>
      </>
    )
  }

  return (
    <>
      <PageHeader
        title="Escalas"
        action={
          (ministries.data?.length ?? 0) > 1 ? (
            <Select value={String(ministryId)} onValueChange={(value) => update({ ministerio: value })}>
              <SelectTrigger className="w-40">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ministries.data?.map((m) => (
                  <SelectItem key={m.id} value={String(m.id)}>
                    {m.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            <Badge variant="outline">{ministries.data?.[0]?.name}</Badge>
          )
        }
      />
      <div className="mb-5 flex items-center justify-between rounded-xl border bg-card p-1.5">
        <Button variant="ghost" size="icon" aria-label="Mês anterior" onClick={() => update({ mes: shiftMonth(month, -1) })}>
          <ChevronLeft />
        </Button>
        <span className="font-medium">{monthLabel(month)}</span>
        <Button variant="ghost" size="icon" aria-label="Próximo mês" onClick={() => update({ mes: shiftMonth(month, 1) })}>
          <ChevronRight />
        </Button>
      </div>
      <ScheduleBoard key={`${ministryId}-${month}`} ministryId={ministryId} month={month} />
    </>
  )
}

function ScheduleBoard({ ministryId, month }: { ministryId: number; month: string }) {
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
      toast.success("Escala gerada. Revise antes de publicar.")
    },
    onError: toastError,
  })
  const discard = useMutation({
    mutationFn: () => api.schedules.discard(ministryId, month),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: queryKeys.schedule(ministryId, month) })
      queryClient.invalidateQueries({ queryKey: queryKeys.schedule(ministryId, month) })
      toast.success("Rascunho descartado")
    },
    onError: toastError,
  })
  const publish = useMutation({
    mutationFn: () => api.schedules.publish(ministryId, month),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.schedule(ministryId, month) })
      toast.success("Escala publicada")
    },
    onError: toastError,
  })

  const missing = schedule.error instanceof ApiError && schedule.error.status === 404
  if (schedule.isPending || ministry.isPending) {
    return <ListSkeleton rows={4} />
  }

  return (
    <div className="flex flex-col gap-5">
      {missing ? (
        <EmptyState icon={CalendarDays} title={`Nenhuma escala para ${monthLabel(month).toLowerCase()}`}>
          <p>Antes de gerar, registre abaixo quem já sabe que não pode neste mês.</p>
          <Button className="mt-4" onClick={() => generate.mutate()} disabled={generate.isPending}>
            <Sparkles />
            {generate.isPending ? "Gerando…" : "Gerar escala"}
          </Button>
        </EmptyState>
      ) : schedule.isError ? (
        <EmptyState icon={CalendarDays} title="Não foi possível carregar a escala">
          {schedule.error.message}
        </EmptyState>
      ) : (
        <>
          <div className="flex flex-wrap items-center gap-2">
            {schedule.data.status === "published" ? (
              <Badge>Publicada</Badge>
            ) : (
              <Badge variant="secondary">Rascunho</Badge>
            )}
            {schedule.data.warnings > 0 && (
              <Badge variant="destructive">
                {schedule.data.warnings} {schedule.data.warnings === 1 ? "vaga para revisar" : "vagas para revisar"}
              </Badge>
            )}
            {schedule.data.status === "draft" && (
              <div className="ml-auto flex flex-wrap justify-end gap-2">
                <ConfirmButton
                  icon={<Trash2 />}
                  label="Descartar"
                  title="Descartar o rascunho?"
                  description="A escala deste mês volta a não existir. Dá para gerar de novo depois."
                  onConfirm={() => discard.mutate()}
                  pending={discard.isPending}
                />
                <ConfirmButton
                  tone="outline"
                  icon={<RefreshCw />}
                  label="Gerar de novo"
                  title="Gerar a escala de novo?"
                  description="As mudanças feitas à mão neste rascunho serão perdidas."
                  onConfirm={() => generate.mutate()}
                  pending={generate.isPending}
                />
                <ConfirmButton
                  tone="primary"
                  icon={<Send />}
                  label="Publicar"
                  title="Publicar a escala?"
                  description="Depois de publicada ela não é mais gerada de novo, e mudanças viram trocas. Os avisos pelo WhatsApp chegam numa próxima etapa."
                  onConfirm={() => publish.mutate()}
                  pending={publish.isPending}
                />
              </div>
            )}
          </div>

          {schedule.data.weeks.map((week) => (
            <section key={week.start} className="overflow-hidden rounded-xl border bg-card">
              <header className="border-b bg-muted/40 px-4 py-2 text-sm text-muted-foreground">
                Semana de {weekRange(week.start)}
              </header>
              {week.services.map((service) => (
                <div key={`${service.date}-${service.serviceId}`} className="border-b px-4 py-3 last:border-b-0">
                  <p className="font-medium">
                    {weekdayOf(service.date)}, {formatDay(service.date)}
                    <span className="font-normal text-muted-foreground">
                      {" "}
                      · {service.startTime.replace(":", "h")} · {service.name}
                    </span>
                    {service.communion && (
                      <Badge variant="outline" className="ml-2 align-middle">
                        🍷 Santa Ceia
                      </Badge>
                    )}
                  </p>
                  <dl className="mt-2 grid gap-1.5">
                    {service.roles.map((role) => (
                      <div key={role.roleId} className="flex items-start gap-2">
                        <dt className="w-20 shrink-0 pt-0.5 text-sm text-muted-foreground">{role.name}</dt>
                        <dd className="flex flex-wrap gap-1.5">
                          {role.slots.map((slot) => (
                            <SlotChip key={slot.assignmentId} slot={slot} onClick={() => setSelection({ service, role, slot })} />
                          ))}
                        </dd>
                      </div>
                    ))}
                  </dl>
                </div>
              ))}
            </section>
          ))}
        </>
      )}

      <UnavailabilityCard list={unavailabilities.data ?? []} defaultDate={range.from} />

      {ministry.data && (
        <SlotDialog
          selection={selection}
          onClose={() => setSelection(undefined)}
          ministry={ministry.data}
          month={month}
          unavailabilities={unavailabilities.data ?? []}
        />
      )}
    </div>
  )
}

const WARNING_LABEL = {
  unavailable: "não pode",
  doubleBooked: "2 funções",
  notMember: "fora da equipe",
} as const

function SlotChip({ slot, onClick }: { slot: ScheduleSlot; onClick: () => void }) {
  const warning = slot.warning && slot.warning !== "empty" ? WARNING_LABEL[slot.warning] : null
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "inline-flex h-7 items-center gap-1 rounded-full border px-2.5 text-sm transition-colors hover:bg-accent",
        slot.warning ? "border-destructive/50 bg-destructive/10 text-destructive" : "border-border bg-secondary"
      )}
    >
      {slot.warning === "empty" ? "Vago" : slot.personName}
      {warning && <span className="text-xs opacity-80">· {warning}</span>}
    </button>
  )
}

function UnavailabilityCard({ list, defaultDate }: { list: Unavailability[]; defaultDate: string }) {
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const remove = useMutation({
    mutationFn: (id: number) => api.unavailabilities.remove(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["unavailabilities"] })
      queryClient.invalidateQueries({ queryKey: ["schedules"] })
    },
    onError: toastError,
  })

  return (
    <section className="rounded-xl border bg-card">
      <header className="flex items-center justify-between gap-3 border-b px-4 py-2.5">
        <h2 className="flex items-center gap-2 font-medium">
          <CalendarOff className="size-4 text-primary" />
          Quem não pode
        </h2>
        <Button size="sm" variant="outline" onClick={() => setOpen(true)}>
          <Plus />
          Adicionar
        </Button>
      </header>
      {list.length === 0 ? (
        <p className="px-4 py-3 text-sm text-muted-foreground">Ninguém avisou que não pode neste período.</p>
      ) : (
        <ul className="divide-y">
          {list.map((u) => (
            <li key={u.id} className="flex items-center gap-3 px-4 py-2.5">
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">{u.personName}</p>
                <p className="text-xs text-muted-foreground">
                  {formatDateRange(u.startDate, u.endDate)} · {REASON_LABEL[u.reason]}
                </p>
              </div>
              <Button
                variant="ghost"
                size="icon"
                aria-label={`Remover indisponibilidade de ${u.personName}`}
                disabled={remove.isPending}
                onClick={() => remove.mutate(u.id)}
              >
                <X />
              </Button>
            </li>
          ))}
        </ul>
      )}
      <UnavailabilityDialog open={open} onOpenChange={setOpen} defaultDate={defaultDate} />
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

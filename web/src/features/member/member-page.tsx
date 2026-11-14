import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { CalendarDays, CalendarOff, Check, Hourglass, Plus, X } from "lucide-react"
import { useState } from "react"
import { toast } from "sonner"

import { ConfirmButton } from "@/components/confirm-button"
import { EmptyState } from "@/components/empty-state"
import { ListSkeleton } from "@/components/list-skeleton"
import { Button } from "@/components/ui/button"
import { api, queryKeys, type MemberSlot, type Unavailability } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { absenceLabel, firstName, formatDay, REASON_LABEL, weekdayOf } from "@/lib/format"
import { cn } from "@/lib/utils"

import { MyAbsenceDialog } from "./my-absence-dialog"

type Service = {
  key: string
  date: string
  startTime: string
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
      ministryName: slot.ministryName,
      communion: slot.communion,
      roles: [slot.roleName],
      slots: [slot],
    })
  }
  return services
}

export function MemberPage() {
  const me = useQuery({ queryKey: queryKeys.me, queryFn: api.me })
  const slots = useQuery({ queryKey: queryKeys.mySlots, queryFn: api.mine.slots })
  const absences = useQuery({ queryKey: queryKeys.myAbsences, queryFn: api.mine.absences })

  return (
    <div className="flex flex-col gap-8">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Oi, {firstName(me.data?.name ?? "")}! 👋</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Aqui ficam as suas escalas. Se não puder ir, é só avisar por aqui.
        </p>
      </div>

      <section className="flex flex-col gap-3">
        <h2 className="font-medium">Suas próximas escalas</h2>
        {slots.isPending ? (
          <ListSkeleton rows={2} />
        ) : !slots.data?.length ? (
          <EmptyState icon={CalendarDays} title="Nada por enquanto">
            Você não está em nenhuma escala publicada nos próximos meses.
          </EmptyState>
        ) : (
          <ul className="flex flex-col gap-3">
            {groupByService(slots.data).map((service) => (
              <ServiceCard key={service.key} service={service} />
            ))}
          </ul>
        )}
      </section>

      <AbsencesSection absences={absences.data} loading={absences.isPending} />
    </div>
  )
}

function ServiceCard({ service }: { service: Service }) {
  const queryClient = useQueryClient()
  const ids = service.slots.map((slot) => slot.assignmentId)
  const declined = service.slots.every((slot) => slot.status === "declined")
  const confirmed = service.slots.every((slot) => slot.status === "confirmed")
  const searching = service.slots.some((slot) => slot.coverOpen)

  const refresh = () => queryClient.invalidateQueries({ queryKey: ["mine"] })
  const confirm = useMutation({
    mutationFn: () => api.mine.confirm(ids),
    onSuccess: () => {
      toast.success(declined ? "Que bom! Você está de volta na escala 🙌" : "Que bom! Tá confirmado 🙌")
      refresh()
    },
    onError: toastError,
  })
  const decline = useMutation({
    mutationFn: () => api.mine.decline(ids.filter((_, i) => service.slots[i].status !== "declined")),
    onSuccess: () => {
      toast.success("Tudo bem, obrigado por avisar! Já estamos procurando quem fique no seu lugar.")
      refresh()
    },
    onError: toastError,
  })
  const day = `${weekdayOf(service.date).toLowerCase()}, ${formatDay(service.date)}`

  return (
    <li className="rounded-xl border bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="font-medium">
            {weekdayOf(service.date)}, {formatDay(service.date)} · {service.startTime.replace(":", "h")}
          </p>
          <p className="text-sm text-muted-foreground">
            {service.ministryName} · {service.roles.join(" e ")}
            {service.communion && " · 🍷 Santa Ceia"}
          </p>
        </div>
        <Status confirmed={confirmed} declined={declined} searching={searching} />
      </div>

      <div className="mt-3 flex flex-wrap gap-2">
        {declined ? (
          <Button size="sm" variant="outline" disabled={confirm.isPending} onClick={() => confirm.mutate()}>
            Consigo ir, afinal
          </Button>
        ) : (
          <>
            {!confirmed && (
              <Button size="sm" disabled={confirm.isPending} onClick={() => confirm.mutate()}>
                <Check />
                Vou estar lá
              </Button>
            )}
            <ConfirmButton
              tone="outline"
              label="Não vou conseguir"
              title={`Não vai conseguir ir ${day.startsWith("sábado") || day.startsWith("domingo") ? "no" : "na"} ${day}?`}
              description="Tudo bem! A gente avisa a liderança e chama alguém da equipe para ficar no seu lugar."
              onConfirm={() => decline.mutate()}
              pending={decline.isPending}
            />
          </>
        )}
      </div>
    </li>
  )
}

function Status({ confirmed, declined, searching }: { confirmed: boolean; declined: boolean; searching: boolean }) {
  const [label, tone, Icon] = declined
    ? searching
      ? ["Procurando substituto", "text-amber-400", Hourglass]
      : ["A liderança já sabe", "text-muted-foreground", CalendarOff]
    : confirmed
      ? ["Confirmado", "text-emerald-400", Check]
      : ["Falta responder", "text-amber-400", Hourglass]
  return (
    <span className={cn("flex shrink-0 items-center gap-1 text-xs font-medium", tone)}>
      <Icon className="size-3.5" />
      {label}
    </span>
  )
}

function AbsencesSection({ absences, loading }: { absences?: Unavailability[]; loading: boolean }) {
  const queryClient = useQueryClient()
  const [adding, setAdding] = useState(false)
  const [coversFor, setCoversFor] = useState<Unavailability>()
  const remove = useMutation({
    mutationFn: (id: number) => api.mine.removeAbsence(id),
    onSuccess: () => {
      toast.success("Removido")
      queryClient.invalidateQueries({ queryKey: ["mine"] })
    },
    onError: toastError,
  })

  return (
    <section className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3">
        <h2 className="font-medium">Quando não posso</h2>
        <Button size="sm" variant="outline" onClick={() => setAdding(true)}>
          <Plus />
          Avisar
        </Button>
      </div>
      {loading ? (
        <ListSkeleton rows={1} />
      ) : !absences?.length ? (
        <p className="text-sm text-muted-foreground">
          Vai viajar, tem plantão ou algo que se repete, como faculdade às quintas? Avise aqui e a gente não coloca você
          na escala nesses dias.
        </p>
      ) : (
        <ul className="divide-y rounded-xl border bg-card">
          {absences.map((u) => (
            <li key={u.id} className="flex items-center gap-3 px-4 py-3">
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">{absenceLabel(u)}</p>
                <p className="text-xs text-muted-foreground">{REASON_LABEL[u.reason]}</p>
              </div>
              {u.scheduled > 0 && (
                <Button size="sm" variant="outline" onClick={() => setCoversFor(u)}>
                  Procurar substituto
                </Button>
              )}
              <Button
                variant="ghost"
                size="icon"
                aria-label={`Remover ${absenceLabel(u)}`}
                disabled={remove.isPending}
                onClick={() => remove.mutate(u.id)}
              >
                <X />
              </Button>
            </li>
          ))}
        </ul>
      )}
      <MyAbsenceDialog open={adding} onOpenChange={setAdding} />
      <MyAbsenceDialog
        open={Boolean(coversFor)}
        onOpenChange={(next) => !next && setCoversFor(undefined)}
        absence={coversFor}
      />
    </section>
  )
}

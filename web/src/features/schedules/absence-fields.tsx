import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import type { Reason, Repeat, UnavailabilityInput } from "@/lib/api"
import { absenceLabel, REASON_LABEL, WEEKDAYS } from "@/lib/format"

export type AbsenceDraft = {
  repeat: Repeat
  startDate: string
  endDate: string
  weekday: string
  weekOfMonth: string
  reason: Reason | ""
}

export function emptyAbsence(date: string): AbsenceDraft {
  return { repeat: "none", startDate: date, endDate: date, weekday: "", weekOfMonth: "", reason: "" }
}

export function absenceReady(draft: AbsenceDraft): boolean {
  if (!draft.reason || !draft.startDate) {
    return false
  }
  switch (draft.repeat) {
    case "none":
      return Boolean(draft.endDate)
    case "weekly":
      return draft.weekday !== ""
    case "monthly":
      return draft.weekday !== "" && draft.weekOfMonth !== ""
  }
}

export function absenceInput(draft: AbsenceDraft): Omit<UnavailabilityInput, "personId"> {
  const repeating = draft.repeat !== "none"
  return {
    repeat: draft.repeat,
    startDate: draft.startDate,
    endDate: draft.endDate,
    reason: draft.reason as Reason,
    weekday: repeating ? Number(draft.weekday) : null,
    weekOfMonth: draft.repeat === "monthly" ? Number(draft.weekOfMonth) : null,
  }
}

const REPEAT_LABEL: Record<Repeat, string> = {
  none: "Não, só nesses dias",
  weekly: "Toda semana",
  monthly: "Uma vez por mês",
}

export function AbsenceFields({ value, onChange }: { value: AbsenceDraft; onChange: (next: AbsenceDraft) => void }) {
  const set = (patch: Partial<AbsenceDraft>) => onChange({ ...value, ...patch })
  const repeating = value.repeat !== "none"

  const changeRepeat = (repeat: Repeat) => {
    set({ repeat, endDate: repeat === "none" ? value.startDate : "" })
  }

  return (
    <>
      <div className="grid gap-2">
        <Label>Se repete?</Label>
        <Select value={value.repeat} onValueChange={(repeat) => changeRepeat(repeat as Repeat)}>
          <SelectTrigger className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(REPEAT_LABEL).map(([repeat, label]) => (
              <SelectItem key={repeat} value={repeat}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {repeating && (
        <div className={value.repeat === "monthly" ? "grid grid-cols-2 gap-3" : "grid gap-2"}>
          {value.repeat === "monthly" && (
            <div className="grid gap-2">
              <Label>Qual</Label>
              <Select value={value.weekOfMonth} onValueChange={(weekOfMonth) => set({ weekOfMonth })}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="Escolha" />
                </SelectTrigger>
                <SelectContent>
                  {[1, 2, 3, 4].map((week) => (
                    <SelectItem key={week} value={String(week)}>
                      {week}º do mês
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          <div className="grid gap-2">
            <Label>Dia da semana</Label>
            <Select value={value.weekday} onValueChange={(weekday) => set({ weekday })}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder="Escolha o dia" />
              </SelectTrigger>
              <SelectContent>
                {WEEKDAYS.map((day, index) => (
                  <SelectItem key={day} value={String(index)}>
                    {day}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
      )}

      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-2">
          <Label htmlFor="absence-from">{repeating ? "A partir de" : "De"}</Label>
          <Input
            id="absence-from"
            type="date"
            value={value.startDate}
            onChange={(e) => {
              const startDate = e.target.value
              const endDate = value.endDate && value.endDate < startDate ? startDate : value.endDate
              set({ startDate, endDate })
            }}
            required
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="absence-to">{repeating ? "Até (opcional)" : "Até"}</Label>
          <Input
            id="absence-to"
            type="date"
            min={value.startDate}
            value={value.endDate}
            onChange={(e) => set({ endDate: e.target.value })}
            required={!repeating}
          />
        </div>
      </div>
      {repeating && absenceReady({ ...value, reason: "outro" }) && (
        <p className="-mt-2 text-sm text-muted-foreground">
          {absenceLabel({ ...absenceInput({ ...value, reason: "outro" }), endDate: value.endDate })}
          {!value.endDate && ", sem data para acabar"}
        </p>
      )}

      <div className="grid gap-2">
        <Label>Motivo</Label>
        <Select value={value.reason} onValueChange={(reason) => set({ reason: reason as Reason })}>
          <SelectTrigger className="w-full">
            <SelectValue placeholder="Escolha o motivo" />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(REASON_LABEL).map(([reason, label]) => (
              <SelectItem key={reason} value={reason}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </>
  )
}

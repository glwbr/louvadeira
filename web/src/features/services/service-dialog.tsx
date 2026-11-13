import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { api, queryKeys, type Service } from "@/lib/api"
import { toastError } from "@/lib/errors"
import { WEEKDAYS } from "@/lib/format"

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  service?: Service
}

export function ServiceDialog({ open, onOpenChange, service }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>{open && <ServiceForm service={service} onDone={() => onOpenChange(false)} />}</DialogContent>
    </Dialog>
  )
}

function ServiceForm({ service, onDone }: { service?: Service; onDone: () => void }) {
  const queryClient = useQueryClient()
  const [name, setName] = useState(service?.name ?? "")
  const [weekday, setWeekday] = useState(String(service?.weekday ?? 0))
  const [startTime, setStartTime] = useState(service?.startTime ?? "19:00")
  const [active, setActive] = useState(service?.active ?? true)
  const [communionWeek, setCommunionWeek] = useState(String(service?.communionWeek ?? "none"))

  const save = useMutation({
    mutationFn: () => {
      const input = {
        name,
        weekday: Number(weekday),
        startTime,
        active,
        communionWeek: communionWeek === "none" ? null : Number(communionWeek),
      }
      return service ? api.services.update(service.id, input) : api.services.create(input)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.services })
      toast.success(service ? "Culto atualizado" : "Culto cadastrado")
      onDone()
    },
    onError: toastError,
  })

  const submit = (event: FormEvent) => {
    event.preventDefault()
    save.mutate()
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>{service ? "Editar culto" : "Novo culto"}</DialogTitle>
        <DialogDescription>Cultos que se repetem toda semana.</DialogDescription>
      </DialogHeader>
      <div className="grid gap-2">
        <Label htmlFor="service-name">Nome</Label>
        <Input
          id="service-name"
          placeholder="Culto de domingo à noite"
          value={name}
          onChange={(e) => setName(e.target.value)}
          autoFocus
          required
        />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-2">
          <Label>Dia</Label>
          <Select value={weekday} onValueChange={setWeekday}>
            <SelectTrigger className="w-full">
              <SelectValue />
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
        <div className="grid gap-2">
          <Label htmlFor="service-time">Horário</Label>
          <Input id="service-time" type="time" value={startTime} onChange={(e) => setStartTime(e.target.value)} required />
        </div>
      </div>
      <div className="grid gap-2">
        <Label>Santa Ceia</Label>
        <Select value={communionWeek} onValueChange={setCommunionWeek}>
          <SelectTrigger className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="none">Não tem Santa Ceia neste culto</SelectItem>
            {[1, 2, 3, 4].map((n) => (
              <SelectItem key={n} value={String(n)}>
                {n}º {WEEKDAYS[Number(weekday)].toLowerCase()} do mês
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {service && (
        <label className="flex items-center justify-between gap-3 rounded-lg border p-3">
          <div>
            <p className="text-sm font-medium">Culto ativo</p>
            <p className="text-xs text-muted-foreground">Desative em vez de apagar, para manter o histórico.</p>
          </div>
          <Switch checked={active} onCheckedChange={setActive} />
        </label>
      )}
      <DialogFooter>
        <Button type="submit" disabled={save.isPending}>
          {save.isPending ? "Salvando…" : "Salvar"}
        </Button>
      </DialogFooter>
    </form>
  )
}

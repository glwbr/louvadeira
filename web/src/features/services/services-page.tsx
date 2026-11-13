import { useQuery } from "@tanstack/react-query"
import { CalendarClock, Plus } from "lucide-react"
import { useState } from "react"

import { EmptyState } from "@/components/empty-state"
import { ListSkeleton } from "@/components/list-skeleton"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { api, queryKeys, type Service } from "@/lib/api"
import { communionLabel, serviceLabel } from "@/lib/format"
import { cn } from "@/lib/utils"
import { ServiceDialog } from "./service-dialog"

export function ServicesPage() {
  const services = useQuery({ queryKey: queryKeys.services, queryFn: api.services.list })
  const [editing, setEditing] = useState<Service | undefined>()
  const [open, setOpen] = useState(false)

  const openDialog = (service?: Service) => {
    setEditing(service)
    setOpen(true)
  }

  return (
    <>
      <PageHeader
        title="Cultos"
        description="A agenda fixa da igreja. Cada ministério escolhe em quais serve."
        action={
          <Button onClick={() => openDialog()}>
            <Plus />
            Novo
          </Button>
        }
      />

      {services.isPending ? (
        <ListSkeleton />
      ) : services.data?.length === 0 ? (
        <EmptyState icon={CalendarClock} title="Nenhum culto cadastrado">
          Cadastre os cultos da semana, como quinta às 19h30 e domingo às 19h.
        </EmptyState>
      ) : (
        <ul className="flex flex-col divide-y rounded-xl border bg-card">
          {services.data?.map((service) => (
            <li key={service.id}>
              <button
                type="button"
                onClick={() => openDialog(service)}
                className={cn(
                  "flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-accent",
                  !service.active && "opacity-60"
                )}
              >
                <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/15 text-primary">
                  <CalendarClock className="size-4" />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium">{service.name}</p>
                  <p className="text-sm text-muted-foreground">{serviceLabel(service)}</p>
                  {communionLabel(service) && (
                    <p className="text-xs text-muted-foreground">🍷 {communionLabel(service)}</p>
                  )}
                </div>
                {!service.active && <Badge variant="secondary">Inativo</Badge>}
              </button>
            </li>
          ))}
        </ul>
      )}

      <ServiceDialog open={open} onOpenChange={setOpen} service={editing} />
    </>
  )
}

import { useQuery } from "@tanstack/react-query"
import { ChevronRight, Church, Plus } from "lucide-react"
import { useState } from "react"
import { Link } from "react-router"

import { EmptyState } from "@/components/empty-state"
import { ListSkeleton } from "@/components/list-skeleton"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { api, queryKeys } from "@/lib/api"
import { rotationLabel } from "@/lib/format"
import { MinistryDialog } from "./ministry-dialog"

export function MinistriesPage() {
  const ministries = useQuery({ queryKey: queryKeys.ministries, queryFn: api.ministries.list })
  const [open, setOpen] = useState(false)

  return (
    <>
      <PageHeader
        title="Ministérios"
        description="Cada ministério tem suas funções, sua equipe e seus cultos."
        action={
          <Button onClick={() => setOpen(true)}>
            <Plus />
            Novo
          </Button>
        }
      />

      {ministries.isPending ? (
        <ListSkeleton />
      ) : ministries.data?.length === 0 ? (
        <EmptyState icon={Church} title="Nenhum ministério ainda">
          Comece pelo louvor: crie o ministério, as funções e coloque a equipe.
        </EmptyState>
      ) : (
        <ul className="flex flex-col gap-2">
          {ministries.data?.map((ministry) => (
            <li key={ministry.id}>
              <Link
                to={`/ministerios/${ministry.id}`}
                className="flex items-center gap-3 rounded-xl border bg-card p-4 transition-colors hover:bg-accent"
              >
                <div className="flex size-10 items-center justify-center rounded-lg bg-primary/15 text-primary">
                  <Church className="size-5" />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium">{ministry.name}</p>
                  <p className="text-sm text-muted-foreground">
                    {ministry.memberCount} {ministry.memberCount === 1 ? "pessoa" : "pessoas"} ·{" "}
                    {rotationLabel(ministry.rotationUnit)}
                  </p>
                </div>
                <ChevronRight className="size-4 text-muted-foreground" />
              </Link>
            </li>
          ))}
        </ul>
      )}

      <MinistryDialog open={open} onOpenChange={setOpen} />
    </>
  )
}

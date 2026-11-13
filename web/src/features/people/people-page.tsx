import { useQuery } from "@tanstack/react-query"
import { Plus, Search, ShieldCheck, Users } from "lucide-react"
import { useMemo, useState } from "react"

import { EmptyState } from "@/components/empty-state"
import { ListSkeleton } from "@/components/list-skeleton"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { api, queryKeys, type Person } from "@/lib/api"
import { formatPhone } from "@/lib/format"
import { PersonDialog } from "./person-dialog"

export function PeoplePage() {
  const people = useQuery({ queryKey: queryKeys.people, queryFn: api.people.list })
  const me = useQuery({ queryKey: queryKeys.me, queryFn: api.me })
  const [search, setSearch] = useState("")
  const [editing, setEditing] = useState<Person | undefined>()
  const [open, setOpen] = useState(false)

  const filtered = useMemo(() => {
    const term = normalize(search)
    return (people.data ?? []).filter((p) => normalize(p.name).includes(term) || p.phone.includes(term))
  }, [people.data, search])

  const openDialog = (person?: Person) => {
    setEditing(person)
    setOpen(true)
  }

  return (
    <>
      <PageHeader
        title="Pessoas"
        description="Todo mundo que serve em algum ministério."
        action={
          <Button onClick={() => openDialog()}>
            <Plus />
            Nova
          </Button>
        }
      />

      {(people.data?.length ?? 0) > 5 && (
        <div className="relative mb-3">
          <Search className="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input className="pl-9" placeholder="Buscar por nome ou telefone" value={search} onChange={(e) => setSearch(e.target.value)} />
        </div>
      )}

      {people.isPending ? (
        <ListSkeleton />
      ) : filtered.length === 0 ? (
        <EmptyState icon={Users} title={search ? "Ninguém com esse nome" : "Nenhuma pessoa cadastrada"}>
          {!search && "Cadastre as pessoas e depois coloque cada uma nos ministérios em que serve."}
        </EmptyState>
      ) : (
        <ul className="flex flex-col divide-y rounded-xl border bg-card">
          {filtered.map((person) => (
            <li key={person.id}>
              <button
                type="button"
                onClick={() => openDialog(person)}
                className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-accent"
              >
                <div className="flex size-9 shrink-0 items-center justify-center rounded-full bg-primary/15 text-sm font-semibold text-primary">
                  {person.name.charAt(0).toUpperCase()}
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium">{person.name}</p>
                  <p className="text-sm text-muted-foreground">{formatPhone(person.phone) || "Sem WhatsApp"}</p>
                </div>
                {person.isAdmin && (
                  <Badge variant="outline">
                    <ShieldCheck />
                    Admin
                  </Badge>
                )}
              </button>
            </li>
          ))}
        </ul>
      )}

      <PersonDialog open={open} onOpenChange={setOpen} person={editing} canArchive={editing?.id !== me.data?.id} />
    </>
  )
}

function normalize(text: string) {
  return text.normalize("NFD").replace(/\p{Diacritic}/gu, "").toLowerCase().trim()
}

import { useStore } from "@tanstack/react-form"
import { useQuery } from "@tanstack/react-query"
import { Plus, Search, SearchX, ShieldCheck, Users, X } from "lucide-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"

import { EmptyState } from "@/components/empty-state"
import { useAppForm } from "@/components/form"
import { ListSkeleton } from "@/components/list-skeleton"
import { Avatar } from "@/components/avatar"
import { ListCard, ListRow } from "@/components/list"
import { AnimatedItem } from "@/components/motion"
import { QueryState } from "@/components/query-state"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { api, queryKeys, type Person } from "@/lib/api"
import { formatPhone } from "@/lib/format"

import { PersonDialog } from "./person-dialog"

const SEARCH_FROM = 6

export function PeoplePage() {
  const { t } = useTranslation()
  const people = useQuery({ queryKey: queryKeys.people, queryFn: api.people.list })
  const me = useQuery({ queryKey: queryKeys.me, queryFn: api.me })
  const filter = useAppForm({ defaultValues: { search: "" } })
  const search = useStore(filter.store, (state) => state.values.search)
  const setSearch = (value: string) => filter.setFieldValue("search", value)
  const [editing, setEditing] = useState<Person | undefined>()
  const [open, setOpen] = useState(false)

  const filtered = useMemo(() => {
    const term = normalize(search)
    const digits = search.replace(/\D/g, "")
    return (people.data ?? []).filter((p) => normalize(p.name).includes(term) || (digits !== "" && p.phone.includes(digits)))
  }, [people.data, search])

  const openDialog = (person?: Person) => {
    setEditing(person)
    setOpen(true)
  }

  return (
    <>
      <PageHeader
        title={t("people.title")}
        description={t("people.lead")}
        action={
          <Button onClick={() => openDialog()}>
            <Plus />
            {t("people.new")}
          </Button>
        }
      />

      {(people.data?.length ?? 0) >= SEARCH_FROM && (
        <div className="relative mb-4">
          <Search className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <filter.Field name="search">
            {(field) => (
              <Input
                type="search"
                className="h-11 rounded-xl pr-11 pl-10"
                aria-label={t("people.search")}
                placeholder={t("people.search")}
                value={field.state.value}
                onChange={(e) => field.handleChange(e.target.value)}
              />
            )}
          </filter.Field>
          {search && (
            <Button
              variant="ghost"
              size="icon-sm"
              className="absolute top-1/2 right-1 -translate-y-1/2"
              aria-label={t("people.clearSearch")}
              onClick={() => setSearch("")}
            >
              <X />
            </Button>
          )}
        </div>
      )}

      <QueryState
        query={people}
        loading={<ListSkeleton rows={5} />}
        isEmpty={(data) => data.length === 0}
        empty={
          <EmptyState icon={Users} title={t("people.emptyTitle")}>
            {t("people.empty")}
          </EmptyState>
        }
      >
        {() =>
          filtered.length === 0 ? (
            <EmptyState
              icon={SearchX}
              title={t("people.noMatchTitle", { term: search.trim() })}
              action={
                <Button variant="outline" onClick={() => setSearch("")}>
                  {t("people.clearSearch")}
                </Button>
              }
            >
              {t("people.noMatch")}
            </EmptyState>
          ) : (
            <ListCard>
              {filtered.map((person) => (
                <AnimatedItem key={person.id}>
                  <ListRow
                    onClick={() => openDialog(person)}
                    leading={<Avatar name={person.name} />}
                    title={<span className="line-clamp-2">{person.name}</span>}
                    subtitle={<span className="tabular">{formatPhone(person.phone) || t("people.noWhatsApp")}</span>}
                    trailing={
                      person.isAdmin && (
                        <Badge variant="brand">
                          <ShieldCheck />
                          {t("people.admin")}
                        </Badge>
                      )
                    }
                  />
                </AnimatedItem>
              ))}
            </ListCard>
          )
        }
      </QueryState>

      <PersonDialog open={open} onOpenChange={setOpen} person={editing} canArchive={editing?.id !== me.data?.id} />
    </>
  )
}

function normalize(text: string) {
  return text.normalize("NFD").replace(/\p{Diacritic}/gu, "").toLowerCase().trim()
}

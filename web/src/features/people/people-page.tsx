import { useStore } from "@tanstack/react-form"
import { useQuery } from "@tanstack/react-query"
import { ChevronRight, Plus, Search, SearchX, ShieldCheck, Users, X } from "lucide-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"

import { EmptyState } from "@/components/empty-state"
import { useAppForm } from "@/components/form"
import { ErrorState } from "@/components/error-state"
import { ListSkeleton } from "@/components/list-skeleton"
import { RevealItem } from "@/components/motion"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { api, queryKeys, type Person } from "@/lib/api"
import { formatPhone, initials } from "@/lib/format"

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

      {people.isPending ? (
        <ListSkeleton rows={5} />
      ) : people.isError ? (
        <ErrorState error={people.error} onRetry={() => void people.refetch()} retrying={people.isFetching} />
      ) : people.data.length === 0 ? (
        <EmptyState icon={Users} title={t("people.emptyTitle")}>
          {t("people.empty")}
        </EmptyState>
      ) : filtered.length === 0 ? (
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
        <ul className="flex flex-col divide-y rounded-2xl border bg-card">
          {filtered.map((person, index) => (
            <RevealItem key={person.id} index={index}>
              <button
                type="button"
                onClick={() => openDialog(person)}
                className="group flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-accent/40"
              >
                <span className="flex size-10 shrink-0 items-center justify-center rounded-full bg-secondary text-sm font-semibold text-secondary-foreground">
                  {initials(person.name)}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="line-clamp-2 font-medium text-pretty">{person.name}</span>
                  <span className="tabular block text-sm text-muted-foreground">{formatPhone(person.phone) || t("people.noWhatsApp")}</span>
                </span>
                {person.isAdmin && (
                  <Badge variant="brand">
                    <ShieldCheck />
                    {t("people.admin")}
                  </Badge>
                )}
                <ChevronRight className="size-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
              </button>
            </RevealItem>
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

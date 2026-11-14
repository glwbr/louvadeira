import { useQuery } from "@tanstack/react-query"
import { Church, Plus } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import { EmptyState } from "@/components/empty-state"
import { ListSkeleton } from "@/components/list-skeleton"
import { IconTile } from "@/components/icon-tile"
import { ListCard, ListRow } from "@/components/list"
import { AnimatedItem } from "@/components/motion"
import { QueryState } from "@/components/query-state"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { queries } from "@/lib/queries"
import { rotationLabel } from "@/lib/format"

import { MinistryDialog } from "./ministry-dialog"

export function MinistriesPage() {
  const { t } = useTranslation()
  const ministries = useQuery(queries.ministries())
  const [open, setOpen] = useState(false)

  const add = (
    <Button onClick={() => setOpen(true)}>
      <Plus />
      {t("ministries.new")}
    </Button>
  )

  return (
    <>
      <PageHeader title={t("ministries.title")} description={t("ministries.lead")} action={add} />

      <QueryState
        query={ministries}
        loading={<ListSkeleton />}
        isEmpty={(data) => data.length === 0}
        empty={
          <EmptyState icon={Church} title={t("ministries.emptyTitle")}>
            {t("ministries.empty")}
          </EmptyState>
        }
      >
        {(data) => (
          <ListCard>
            {data.map((ministry) => (
              <AnimatedItem key={ministry.id}>
                <ListRow
                  to={`/ministerios/${ministry.id}`}
                  className="py-4"
                  leading={<IconTile icon={Church} />}
                  title={ministry.name}
                  subtitle={`${t("ministries.people", { count: ministry.memberCount })} · ${rotationLabel(ministry.rotationUnit)}`}
                />
              </AnimatedItem>
            ))}
          </ListCard>
        )}
      </QueryState>

      <MinistryDialog open={open} onOpenChange={setOpen} />
    </>
  )
}

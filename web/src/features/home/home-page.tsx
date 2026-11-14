import { useQuery } from "@tanstack/react-query"
import { CalendarClock, CalendarHeart, ChevronRight, Church, MessageCircle, Users, type LucideIcon } from "lucide-react"
import { Link } from "react-router"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { api, queryKeys, type Health } from "@/lib/api"
import { firstName } from "@/lib/format"

const MODE_LABEL: Record<Health["whatsapp"]["mode"], string> = {
  off: "Desligado",
  test: "Modo de teste",
  live: "Ativo",
  "": "Desconhecido",
}

export function HomePage() {
  const me = useQuery({ queryKey: queryKeys.me, queryFn: api.me })
  const ministries = useQuery({ queryKey: queryKeys.ministries, queryFn: api.ministries.list })
  const people = useQuery({ queryKey: queryKeys.people, queryFn: api.people.list })
  const services = useQuery({ queryKey: queryKeys.services, queryFn: api.services.list })
  const health = useQuery({ queryKey: queryKeys.health, queryFn: api.health, refetchInterval: 10_000 })

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Olá, {firstName(me.data?.name ?? "")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">Cadastro dos ministérios da igreja.</p>
      </div>

      <Link
        to="/minha-escala"
        className="flex items-center justify-between rounded-xl border bg-card px-4 py-3 transition-colors hover:bg-accent"
      >
        <span className="flex items-center gap-2 text-sm font-medium">
          <CalendarHeart className="size-4 text-primary" />
          Minhas escalas
        </span>
        <ChevronRight className="size-4 text-muted-foreground" />
      </Link>

      <div className="grid gap-3 sm:grid-cols-3">
        <Shortcut to="/ministerios" icon={Church} label="Ministérios" count={ministries.data?.length} />
        <Shortcut to="/pessoas" icon={Users} label="Pessoas" count={people.data?.length} />
        <Shortcut to="/cultos" icon={CalendarClock} label="Cultos" count={services.data?.filter((s) => s.active).length} />
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <MessageCircle className="size-4 text-primary" />
            WhatsApp
          </CardTitle>
          <CardDescription>Em modo de teste, só envia para o grupo e os números liberados.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          {health.isError && <Badge variant="destructive">Servidor fora do ar</Badge>}
          {health.data && (
            <>
              <Badge variant="outline">{MODE_LABEL[health.data.whatsapp.mode]}</Badge>
              <Badge variant={health.data.whatsapp.linked ? "default" : "secondary"}>
                {health.data.whatsapp.linked ? "Número vinculado" : "Nenhum número vinculado"}
              </Badge>
              {health.data.whatsapp.linked && (
                <Badge variant={health.data.whatsapp.connected ? "default" : "destructive"}>
                  {health.data.whatsapp.connected ? "Conectado" : "Desconectado"}
                </Badge>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function Shortcut({ to, icon: Icon, label, count }: { to: string; icon: LucideIcon; label: string; count?: number }) {
  return (
    <Link
      to={to}
      className="flex items-center gap-3 rounded-xl border bg-card p-4 transition-colors hover:bg-accent"
    >
      <div className="flex size-10 items-center justify-center rounded-lg bg-primary/15 text-primary">
        <Icon className="size-5" />
      </div>
      <div className="flex-1">
        <p className="text-sm text-muted-foreground">{label}</p>
        <p className="text-xl font-semibold">{count ?? "–"}</p>
      </div>
      <ChevronRight className="size-4 text-muted-foreground" />
    </Link>
  )
}

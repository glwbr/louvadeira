import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Loader2, MessageCircle } from "lucide-react"
import { useEffect, useRef, useState, type FormEvent, type ReactNode } from "react"
import { useNavigate, useSearchParams } from "react-router"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api, queryKeys } from "@/lib/api"

export function SignInPage() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [error, setError] = useState<string | null>(null)
  const started = useRef(false)
  const token = params.get("token")

  useEffect(() => {
    // NOTE: StrictMode runs effects twice in development; the link is single-use, so redeem it only once.
    if (!token || started.current) {
      return
    }
    started.current = true
    api
      .redeem(token)
      .then((person) => {
        queryClient.setQueryData(queryKeys.me, person)
        navigate(person.isAdmin ? "/" : "/minha-escala", { replace: true })
      })
      .catch((err: Error) => setError(err.message))
  }, [token, navigate, queryClient])

  if (token && !error) {
    return (
      <Centered>
        <Loader2 className="size-6 animate-spin text-primary" />
        <h1 className="text-xl font-semibold">Entrando…</h1>
      </Centered>
    )
  }

  return (
    <Centered>
      <div className="flex size-14 items-center justify-center rounded-2xl bg-primary/15 text-primary">
        <MessageCircle className="size-6" />
      </div>
      <h1 className="text-xl font-semibold">{error ? "Esse link não vale mais" : "Entrar na louvadeira"}</h1>
      <p className="text-sm text-muted-foreground">
        {error
          ? "Cada link funciona uma vez só. Digite seu WhatsApp que a gente manda um novo."
          : "Digite seu WhatsApp e a gente manda um link de acesso por lá."}
      </p>
      <PhoneForm />
    </Centered>
  )
}

function PhoneForm() {
  const [phone, setPhone] = useState("")
  const request = useMutation({ mutationFn: () => api.requestLink(phone) })

  const submit = (event: FormEvent) => {
    event.preventDefault()
    request.mutate()
  }

  if (request.isSuccess) {
    return (
      <p className="rounded-xl border bg-card px-4 py-3 text-sm">
        Pronto! Se esse número estiver na equipe, o link chega no seu WhatsApp em instantes. 📲
      </p>
    )
  }

  return (
    <form onSubmit={submit} className="flex w-full flex-col gap-3 text-left">
      <div className="grid gap-2">
        <Label htmlFor="phone">WhatsApp com DDD</Label>
        <Input
          id="phone"
          type="tel"
          inputMode="tel"
          autoComplete="tel"
          placeholder="(75) 99999-9999"
          value={phone}
          onChange={(e) => setPhone(e.target.value)}
          required
        />
      </div>
      {request.isError && <p className="text-sm text-destructive">{request.error.message}</p>}
      <Button type="submit" disabled={request.isPending || phone.replace(/\D/g, "").length < 10}>
        {request.isPending ? "Enviando…" : "Receber link"}
      </Button>
    </form>
  )
}

function Centered({ children }: { children: ReactNode }) {
  return (
    <main className="mx-auto flex min-h-svh max-w-sm flex-col items-center justify-center gap-4 px-6 text-center">
      {children}
    </main>
  )
}

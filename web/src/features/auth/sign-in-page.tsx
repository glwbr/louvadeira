import { useQueryClient } from "@tanstack/react-query"
import { Loader2, LockKeyhole } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { useNavigate, useSearchParams } from "react-router"

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
        navigate("/", { replace: true })
      })
      .catch((err: Error) => setError(err.message))
  }, [token, navigate, queryClient])

  const message = !token
    ? "Este endereço precisa de um link de acesso. Peça um novo ao administrador."
    : error
      ? `${error}. Peça um novo ao administrador.`
      : null

  return (
    <main className="mx-auto flex min-h-svh max-w-sm flex-col items-center justify-center gap-4 px-6 text-center">
      <div className="flex size-14 items-center justify-center rounded-2xl bg-primary/15 text-primary">
        {message ? <LockKeyhole className="size-6" /> : <Loader2 className="size-6 animate-spin" />}
      </div>
      <h1 className="text-xl font-semibold">{message ? "Não deu para entrar" : "Entrando…"}</h1>
      {message && <p className="text-sm text-muted-foreground">{message}</p>}
    </main>
  )
}

import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import { MotionConfig } from "motion/react"
import { BrowserRouter } from "react-router"
import { registerSW } from "virtual:pwa-register"

import "./index.css"
import "@/lib/i18n"
import App from "./App.tsx"
import { ThemeProvider } from "@/components/theme-provider.tsx"
import { Toaster } from "@/components/ui/sonner"
import { ApiError } from "@/lib/api"

// NOTE: With autoUpdate, registerSW reloads the page once a new version is installed, so nobody keeps an old build.
registerSW({ immediate: true })

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // NOTE: Client errors (4xx) will not change on retry; offline (0) and server errors get two more tries.
      retry: (failures, error) =>
        !(error instanceof ApiError && error.status >= 400 && error.status < 500) && failures < 2,
      staleTime: 30_000,
    },
  },
})

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <MotionConfig reducedMotion="user">
      <ThemeProvider>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter>
            <App />
          </BrowserRouter>
          <Toaster position="top-center" richColors />
        </QueryClientProvider>
      </ThemeProvider>
    </MotionConfig>
  </StrictMode>
)

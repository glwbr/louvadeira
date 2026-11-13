import { resolve } from "node:path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"
import { VitePWA } from "vite-plugin-pwa"

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: "autoUpdate",
      includeAssets: ["favicon.svg", "apple-touch-icon.png"],
      manifest: {
        name: "louvadeira",
        short_name: "louvadeira",
        description: "Escalas dos ministérios",
        lang: "pt-BR",
        start_url: "/",
        display: "standalone",
        background_color: "#0f1b3d",
        theme_color: "#0f1b3d",
        icons: [
          { src: "pwa-192x192.png", sizes: "192x192", type: "image/png" },
          { src: "pwa-512x512.png", sizes: "512x512", type: "image/png" },
          { src: "maskable-512x512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
        ],
      },
      workbox: {
        navigateFallbackDenylist: [/^\/api\//],
      },
    }),
  ],
  resolve: {
    alias: {
      "@": resolve(import.meta.dirname, "./src"),
    },
  },
  server: {
    proxy: {
      "/api": "http://localhost:8090",
    },
  },
})

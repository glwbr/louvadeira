import i18n from "i18next"
import { initReactI18next } from "react-i18next"

import { ptBR } from "@/locales/pt-BR"

export const LOCALE = "pt-BR"

/**
 * NOTE: The app speaks only Portuguese for now; the catalog lives in one place so wording stays consistent and a
 * second language would only need a new resource file. Resources are bundled, so init is synchronous.
 */
void i18n.use(initReactI18next).init({
  lng: LOCALE,
  fallbackLng: LOCALE,
  resources: { [LOCALE]: { translation: ptBR } },
  interpolation: { escapeValue: false },
  initAsync: false,
})

export { i18n }
export const t = i18n.t.bind(i18n)

import { createElement, lazy, type ComponentType } from "react"

/**
 * NOTE: A screen loaded on demand that, once its file is in, renders straight away. React.lazy suspends on its first
 * render even when the file is already loaded, and a screen that suspends keeps its fallback for at least 300 ms:
 * a blank gap between two screens.
 */
export function preloadable(load: () => Promise<{ default: ComponentType }>) {
  let loaded: ComponentType | undefined
  const preload = () =>
    load().then((module) => {
      loaded = module.default
      return module
    })
  const Lazy = lazy(preload)
  function Screen() {
    return createElement(loaded ?? Lazy)
  }
  return Object.assign(Screen, { preload })
}

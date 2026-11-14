/**
 * NOTE: Marks <html data-input="keyboard"> while the person moves with the keyboard and "pointer" once they tap or
 * click. Radix tabs move focus themselves, which the browser reads as keyboard focus even after a tap; their ring
 * checks this mark instead, so it only shows to someone actually using the keyboard.
 */
export function trackInputModality() {
  const root = document.documentElement
  document.addEventListener(
    "keydown",
    (event) => {
      if (event.key === "Tab" || event.key.startsWith("Arrow") || event.key === "Home" || event.key === "End") {
        root.dataset.input = "keyboard"
      }
    },
    true
  )
  document.addEventListener("pointerdown", () => (root.dataset.input = "pointer"), true)
}

/**
 * NOTE: Runs a task once the browser is idle, or after a short wait where idle callbacks do not exist (Safari).
 * Returns a function that cancels it.
 */
export function whenIdle(task: () => void): () => void {
  if (typeof requestIdleCallback === "function") {
    const handle = requestIdleCallback(task, { timeout: 3000 })
    return () => cancelIdleCallback(handle)
  }
  const handle = setTimeout(task, 1500)
  return () => clearTimeout(handle)
}

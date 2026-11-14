import { useState } from "react"

/**
 * NOTE: Keeps showing the last value while it is being cleared, so a dialog closing over a selection keeps its content
 * through the closing animation instead of going blank or jumping to another step.
 */
export function useRetained<T>(value: T | undefined): T | undefined {
  const [kept, setKept] = useState(value)
  if (value !== undefined && value !== kept) {
    setKept(value)
  }
  return value ?? kept
}

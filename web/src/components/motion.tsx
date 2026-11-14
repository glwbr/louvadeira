import { motion, type HTMLMotionProps, type Transition } from "motion/react"

export const EASE_OUT = [0.16, 1, 0.3, 1] as const

export const quick: Transition = { duration: 0.22, ease: EASE_OUT }

export const spring: Transition = { type: "spring", bounce: 0.18, duration: 0.45 }

/**
 * CONSTRAINT: Lists stagger only their first few rows so a long list never makes anyone wait for the last one.
 */
const STAGGER_STEP = 0.03
const STAGGER_CAP = 8

export function staggerDelay(index: number): number {
  return Math.min(index, STAGGER_CAP) * STAGGER_STEP
}

/**
 * NOTE: The arrival of a screen or a block that just loaded: a short rise from already-almost-visible, so a slow
 * device never shows an empty page.
 */
export function Reveal({ delay = 0, ...props }: HTMLMotionProps<"div"> & { delay?: number }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ ...quick, delay }}
      {...props}
    />
  )
}

export function RevealItem({ index, ...props }: HTMLMotionProps<"li"> & { index: number }) {
  return (
    <motion.li
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ ...quick, delay: staggerDelay(index) }}
      {...props}
    />
  )
}

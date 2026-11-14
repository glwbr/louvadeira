import { AnimatePresence, motion, type HTMLMotionProps, type Transition } from "motion/react"
import { useState, type ReactNode } from "react"

export const EASE_OUT = [0.16, 1, 0.3, 1] as const

export const quick: Transition = { duration: 0.22, ease: EASE_OUT }

export const spring: Transition = { type: "spring", bounce: 0.18, duration: 0.45 }

/**
 * NOTE: Lists animate only what changes after they first show: a row that is added grows open and a removed one
 * folds shut, so its neighbours move with it. Nothing animates when a screen opens.
 */
export function AnimatedList({ children }: { children: ReactNode }) {
  return <AnimatePresence initial={false}>{children}</AnimatePresence>
}

/**
 * CONSTRAINT: Rows animate their own height, not their position. A position animation also fires when something
 * above the list changes size and would slide the row in from where it used to be, clipped by the card.
 */
export function AnimatedItem(props: HTMLMotionProps<"li">) {
  return (
    <motion.li
      initial={{ opacity: 0, height: 0, overflow: "hidden" }}
      animate={{ opacity: 1, height: "auto", transitionEnd: { overflow: "visible" } }}
      exit={{ opacity: 0, height: 0, overflow: "hidden", transition: { duration: 0.18, ease: EASE_OUT } }}
      transition={quick}
      {...props}
    />
  )
}

/**
 * NOTE: Swaps one view for another (a month, an empty state for a board). The old view goes at once, since it was
 * already dimmed while the new one loaded, and the new one slides in from the side it comes from. Two views never
 * overlap, and the first view on screen does not animate.
 */
export function Swap({ id, direction = 0, children, className }: { id: string; direction?: number; children: ReactNode; className?: string }) {
  const [first] = useState(id)
  return (
    <motion.div
      key={id}
      initial={id === first ? false : { opacity: 0, x: direction * 16 }}
      animate={{ opacity: 1, x: 0 }}
      transition={quick}
      className={className}
    >
      {children}
    </motion.div>
  )
}

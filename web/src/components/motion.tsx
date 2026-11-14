import { AnimatePresence, motion, PresenceContext, type HTMLMotionProps, type Transition } from "motion/react"
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
export function AnimatedItem({ children, ...props }: HTMLMotionProps<"li"> & { children?: ReactNode }) {
  return (
    <motion.li
      initial={{ opacity: 0, height: 0, overflow: "hidden" }}
      animate={{ opacity: 1, height: "auto", transitionEnd: { overflow: "visible" } }}
      exit={{ opacity: 0, height: 0, overflow: "hidden", transition: { duration: 0.18, ease: EASE_OUT } }}
      transition={quick}
      {...props}
    >
      <Isolate>{children}</Isolate>
    </motion.li>
  )
}

/**
 * CONSTRAINT: AnimatePresence with initial={false} blocks the entrance of every motion element inside the children it
 * starts with, even one that mounts much later (a tab panel, a month sliding in). Isolate ends that reach, so only the
 * wrapper itself skips its first entrance.
 */
export function Isolate({ children }: { children: ReactNode }) {
  return <PresenceContext.Provider value={null}>{children}</PresenceContext.Provider>
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

/**
 * NOTE: Fade through between screens, the pattern for moving across top-level destinations: the old screen fades out
 * fast, the new one fades in while growing slightly from its top, and the page starts scrolled to the top. Under
 * reduced motion only the fade remains.
 */
export function ScreenTransition({ id, children }: { id: string; children: ReactNode }) {
  return (
    <AnimatePresence mode="wait" initial={false} onExitComplete={() => window.scrollTo(0, 0)}>
      <motion.div
        key={id}
        className="origin-top"
        initial={{ opacity: 0, scale: 0.985 }}
        animate={{ opacity: 1, scale: 1, transition: { duration: 0.21, ease: EASE_OUT } }}
        exit={{ opacity: 0, transition: { duration: 0.09, ease: "easeIn" } }}
      >
        <Isolate>{children}</Isolate>
      </motion.div>
    </AnimatePresence>
  )
}

import * as React from "react"
import { cn } from "cn"
import { motion } from "motion/react"
import { Tabs as TabsPrimitive } from "radix-ui"

import { quick, spring } from "@/components/motion"

type TabsState = { value: string; direction: number; id: string }

const TabsContext = React.createContext<TabsState | null>(null)

/**
 * NOTE: Tabs keeps the chosen value itself so the indicator can slide to it and the new panel can come in from the
 * side of the tab that was chosen. Direction is 0 until the first change, so nothing moves when the page opens.
 */
function Tabs({
  className,
  orientation = "horizontal",
  value,
  defaultValue,
  onValueChange,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Root>) {
  const id = React.useId()
  const root = React.useRef<HTMLDivElement>(null)
  const [inner, setInner] = React.useState(defaultValue ?? "")
  const [direction, setDirection] = React.useState(0)
  const current = value ?? inner

  const change = (next: string) => {
    const order = [...(root.current?.querySelectorAll<HTMLElement>("[data-tab-value]") ?? [])].map((el) => el.dataset.tabValue)
    setDirection(Math.sign(order.indexOf(next) - order.indexOf(current)))
    setInner(next)
    onValueChange?.(next)
  }

  return (
    <TabsContext.Provider value={{ value: current, direction, id }}>
      <TabsPrimitive.Root
        ref={root}
        data-slot="tabs"
        data-orientation={orientation}
        orientation={orientation}
        value={current}
        onValueChange={change}
        className={cn("group/tabs flex gap-2 data-horizontal:flex-col", className)}
        {...props}
      />
    </TabsContext.Provider>
  )
}

function TabsList({ className, ...props }: React.ComponentProps<typeof TabsPrimitive.List>) {
  return (
    <TabsPrimitive.List
      data-slot="tabs-list"
      className={cn(
        "group/tabs-list inline-flex w-fit items-center justify-center rounded-xl bg-muted p-1 text-muted-foreground group-data-horizontal/tabs:h-11 group-data-vertical/tabs:h-fit group-data-vertical/tabs:flex-col",
        className
      )}
      {...props}
    />
  )
}

function TabsTrigger({ className, value, children, ...props }: React.ComponentProps<typeof TabsPrimitive.Trigger>) {
  const tabs = React.useContext(TabsContext)
  const active = tabs?.value === value
  return (
    <TabsPrimitive.Trigger
      data-slot="tabs-trigger"
      data-tab-value={value}
      value={value}
      className={cn(
        "relative inline-flex h-full min-w-0 flex-1 items-center justify-center rounded-lg px-2 py-0.5 text-sm font-medium whitespace-nowrap text-foreground/60 transition-colors group-data-vertical/tabs:w-full group-data-vertical/tabs:justify-start hover:text-foreground focus-visible:outline-none in-data-[input=keyboard]:focus-visible:ring-[3px] in-data-[input=keyboard]:focus-visible:ring-ring/50 in-data-[input=keyboard]:focus-visible:outline-1 in-data-[input=keyboard]:focus-visible:outline-solid in-data-[input=keyboard]:focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50 data-active:text-foreground dark:text-muted-foreground dark:hover:text-foreground dark:data-active:text-foreground [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        className
      )}
      {...props}
    >
      {active && tabs && (
        <motion.span
          layoutId={`${tabs.id}-indicator`}
          transition={spring}
          aria-hidden
          className="absolute inset-0 rounded-lg bg-background shadow-sm dark:border dark:border-input dark:bg-input/30"
        />
      )}
      <span className="relative inline-flex min-w-0 items-center gap-1.5">{children}</span>
    </TabsPrimitive.Trigger>
  )
}

function TabsContent({ className, children, ...props }: React.ComponentProps<typeof TabsPrimitive.Content>) {
  const tabs = React.useContext(TabsContext)
  const direction = tabs?.direction ?? 0
  return (
    <TabsPrimitive.Content data-slot="tabs-content" className={cn("flex-1 overflow-x-clip text-sm outline-none", className)} {...props}>
      <motion.div
        initial={direction === 0 ? false : { opacity: 0, x: direction * 24 }}
        animate={{ opacity: 1, x: 0 }}
        transition={quick}
      >
        {children}
      </motion.div>
    </TabsPrimitive.Content>
  )
}

export { Tabs, TabsList, TabsTrigger, TabsContent }

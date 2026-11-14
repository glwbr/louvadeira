import type { LucideProps } from "lucide-react"
import { createElement } from "react"

import { roleIcon } from "@/lib/roles"

export function RoleIcon({ name, ...props }: { name: string } & LucideProps) {
  return createElement(roleIcon(name), props)
}

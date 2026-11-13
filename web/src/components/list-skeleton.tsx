import { Skeleton } from "@/components/ui/skeleton"

export function ListSkeleton({ rows = 3 }: { rows?: number }) {
  return (
    <div className="flex flex-col gap-2">
      {Array.from({ length: rows }, (_, i) => (
        <Skeleton key={i} className="h-16 w-full" />
      ))}
    </div>
  )
}

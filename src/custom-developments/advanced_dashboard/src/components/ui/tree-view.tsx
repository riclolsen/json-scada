import * as React from "react"
import { ChevronRightIcon, ChevronDownIcon } from "@radix-ui/react-icons"
import { cn } from "../../lib/utils"

interface TreeViewProps {
  children: React.ReactNode
  className?: string
}

interface TreeItemProps {
  children?: React.ReactNode
  title: string
  defaultExpanded?: boolean
  className?: string
  onClick?: () => void
  loading?: boolean
}

export function TreeView({ children, className }: TreeViewProps) {
  return (
    <div className={cn("text-sm", className)}>
      {children}
    </div>
  )
}

export function TreeItem({ 
  children, 
  title, 
  defaultExpanded = false,
  className,
  onClick,
  loading = false
}: TreeItemProps) {
  const [expanded, setExpanded] = React.useState(defaultExpanded)
  const hasChildren = Boolean(children)

  const handleClick = (e: React.MouseEvent) => {
    if (hasChildren) {
      e.stopPropagation()
      setExpanded(!expanded)
    }
    onClick?.()
  }

  return (
    <div className={cn("select-none", className)}>
      <div
        onClick={handleClick}
        className="flex items-center gap-2 rounded px-2 py-1 hover:bg-gray-700 cursor-pointer"
      >
        {hasChildren ? (
          expanded ? (
            <ChevronDownIcon className="h-4 w-4" />
          ) : (
            <ChevronRightIcon className="h-4 w-4" />
          )
        ) : (
          <div className="w-4" />
        )}
        <span>{title}</span>
        {loading && (
          <div className="ml-2 h-4 w-4 animate-spin rounded-full border-2 border-gray-500 border-t-transparent" />
        )}
      </div>
      {hasChildren && expanded && (
        <div className="ml-4 border-l border-gray-600 pl-2">
          {children}
        </div>
      )}
    </div>
  )
}

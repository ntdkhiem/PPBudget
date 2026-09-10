import * as React from "react"
import { cn } from "@/lib/utils"

interface PageContainerProps extends React.HTMLAttributes<HTMLDivElement> {
  maxWidth?: "3xl" | "4xl" | "5xl" | "6xl" | "7xl" | "full";
}

export function PageContainer({ children, className, maxWidth = "5xl", ...props }: PageContainerProps) {
  const maxWidthClass = {
    "3xl": "max-w-3xl",
    "4xl": "max-w-4xl",
    "5xl": "max-w-5xl",
    "6xl": "max-w-6xl",
    "7xl": "max-w-7xl",
    "full": "max-w-full",
  }[maxWidth];

  return (
    <div 
      className={cn(`${maxWidthClass} mx-auto space-y-8 pb-12 w-full px-4 sm:px-6 lg:px-8`, className)}
      {...props}
    >
      {children}
    </div>
  )
}

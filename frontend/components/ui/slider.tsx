"use client"

import * as React from "react"
import { Slider as SliderPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"

function Slider({
  className,
  ...props
}: React.ComponentProps<typeof SliderPrimitive.Root>) {
  const thumbCount = Array.isArray(props.value ?? props.defaultValue)
    ? (props.value ?? props.defaultValue ?? []).length
    : 1

  return (
    <SliderPrimitive.Root
      data-slot="slider"
      className={cn(
        "relative flex w-full touch-none select-none items-center disabled:opacity-50",
        className
      )}
      {...props}
    >
      <SliderPrimitive.Track
        data-slot="slider-track"
        className="relative h-1.5 w-full grow overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800"
      >
        <SliderPrimitive.Range
          data-slot="slider-range"
          className="absolute h-full bg-indigo-500"
        />
      </SliderPrimitive.Track>
      {Array.from({ length: thumbCount }, (_, i) => (
        <SliderPrimitive.Thumb
          key={i}
          data-slot="slider-thumb"
          className="block size-4 shrink-0 rounded-full border-2 border-indigo-500 bg-white shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500/40 disabled:pointer-events-none dark:bg-slate-900"
        />
      ))}
    </SliderPrimitive.Root>
  )
}

export { Slider }

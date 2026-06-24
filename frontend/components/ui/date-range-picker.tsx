"use client"

import * as React from "react"
import { format, subDays, startOfMonth, endOfMonth, subMonths, startOfYear, endOfYear, subYears, addMonths } from "date-fns"
import { Calendar as CalendarIcon } from "lucide-react"
import { DateRange } from "react-day-picker"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Calendar } from "@/components/ui/calendar"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"

const PRESETS = [
  {
    name: "Today",
    getValue: () => ({ from: new Date(), to: new Date() }),
  },
  {
    name: "Last 7 days",
    getValue: () => ({ from: subDays(new Date(), 6), to: new Date() }),
  },
  {
    name: "Last 30 days",
    getValue: () => ({ from: subDays(new Date(), 29), to: new Date() }),
  },
  {
    name: "Month to date",
    getValue: () => ({ from: startOfMonth(new Date()), to: new Date() }),
  },
  {
    name: "Last month",
    getValue: () => {
      const lastMonth = subMonths(new Date(), 1)
      return { from: startOfMonth(lastMonth), to: endOfMonth(lastMonth) }
    },
  },
  {
    name: "Next month",
    getValue: () => {
      const nextMonth = addMonths(new Date(), 1)
      return { from: startOfMonth(nextMonth), to: endOfMonth(nextMonth) }
    },
  },
  {
    name: "Year to date",
    getValue: () => ({ from: startOfYear(new Date()), to: new Date() }),
  },
  {
    name: "Previous year",
    getValue: () => {
      const prevYear = subYears(new Date(), 1)
      return { from: startOfYear(prevYear), to: endOfYear(prevYear) }
    },
  },
  {
    name: "Everything",
    getValue: () => undefined,
  },
]

export function DatePickerWithRange({
  className,
  date,
  setDate,
}: {
  className?: string
  date: DateRange | undefined
  setDate: (date: DateRange | undefined) => void
}) {
  const [isOpen, setIsOpen] = React.useState(false);

  return (
    <div className={cn("grid gap-2", className)}>
      <Popover open={isOpen} onOpenChange={setIsOpen}>
        <PopoverTrigger asChild>
          <Button
            id="date"
            variant={"outline"}
            className={cn(
              "w-full justify-start text-left font-normal text-xs bg-white/50 dark:bg-slate-900/50 backdrop-blur-sm border-slate-200/60 dark:border-slate-800/60 hover:bg-white dark:hover:bg-slate-900 shadow-sm",
              !date && "text-muted-foreground"
            )}
          >
            <CalendarIcon className="mr-2 h-4 w-4 shrink-0" />
            <span className="truncate">
              {date?.from ? (
                date.to ? (
                  <>
                    {format(date.from, "LLL dd")} - {format(date.to, "LLL dd, y")}
                  </>
                ) : (
                  format(date.from, "LLL dd, y")
                )
              ) : (
                "Everything"
              )}
            </span>
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-auto p-0 flex flex-col md:flex-row" align="start">
          <div className="flex flex-col gap-1 border-b md:border-b-0 md:border-r border-slate-200 dark:border-slate-800 p-2 md:w-[150px] overflow-y-auto max-h-[300px] md:max-h-[350px]">
            {PRESETS.map((preset) => (
              <Button
                key={preset.name}
                variant="ghost"
                className="justify-start text-xs font-normal"
                onClick={() => {
                  setDate(preset.getValue())
                  setIsOpen(false)
                }}
              >
                {preset.name}
              </Button>
            ))}
          </div>
          <Calendar
            mode="range"
            defaultMonth={date?.from}
            selected={date}
            onSelect={setDate}
            numberOfMonths={2}
          />
        </PopoverContent>
      </Popover>
    </div>
  )
}

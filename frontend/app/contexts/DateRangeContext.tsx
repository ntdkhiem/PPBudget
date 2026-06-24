"use client";

import { createContext, useContext, useState, ReactNode } from "react";
import { startOfMonth, endOfMonth } from "date-fns";
import { DateRange } from "react-day-picker";

interface DateRangeContextType {
  date: DateRange | undefined;
  setDate: (date: DateRange | undefined) => void;
}

const DateRangeContext = createContext<DateRangeContextType | undefined>(undefined);

export function DateRangeProvider({ children }: { children: ReactNode }) {
  const [date, setDate] = useState<DateRange | undefined>({
    from: startOfMonth(new Date()),
    to: endOfMonth(new Date()),
  });

  return (
    <DateRangeContext.Provider value={{ date, setDate }}>
      {children}
    </DateRangeContext.Provider>
  );
}

export function useDateRange() {
  const context = useContext(DateRangeContext);
  if (context === undefined) {
    throw new Error("useDateRange must be used within a DateRangeProvider");
  }
  return context;
}

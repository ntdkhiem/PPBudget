"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { useState, ReactNode } from "react";

import { toast } from "sonner";
import { QueryCache, MutationCache } from "@tanstack/react-query";
import { DateRangeProvider } from "./contexts/DateRangeContext";

export function Providers({ children }: { children: ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 60 * 1000, // 1 minute
            retry: false, // Fail fast instead of retrying 3 times with exponential backoff
          },
        },
        mutationCache: new MutationCache({
          onError: (error: any) => {
            toast.error(error.message || "An unexpected error occurred");
          },
        }),
      })
  );

  return (
    <QueryClientProvider client={queryClient}>
      <DateRangeProvider>
        {children}
      </DateRangeProvider>
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  );
}

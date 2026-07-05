import { useQuery } from "@tanstack/react-query";
import { useState, useEffect } from "react";
import { apiFetch, Transaction } from "@/lib/api";

export function useTransactionSearch(
  initialQuery: string = "",
  startDate?: string,
  endDate?: string
) {
  const [searchQuery, setSearchQuery] = useState(initialQuery);
  const [debouncedQuery, setDebouncedQuery] = useState(initialQuery);

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedQuery(searchQuery);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchQuery]);

  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || undefined : undefined;

  const { data: transactions, isLoading } = useQuery<Transaction[]>({
    queryKey: ["transactions-search", debouncedQuery, startDate, endDate],
    queryFn: async () => {
      let url = "/transactions?";
      const params = new URLSearchParams();
      if (debouncedQuery) {
        params.append("search", debouncedQuery);
      }
      if (startDate && !debouncedQuery) { // Only restrict by date if not explicitly searching
        params.append("start_date", startDate);
      }
      if (endDate && !debouncedQuery) {
        params.append("end_date", endDate);
      }
      
      const res = await apiFetch<Transaction[]>(`${url}${params.toString()}`, {}, token);
      return res || [];
    },
    // We only enable this query if we have a token
    enabled: !!token,
    staleTime: 60 * 1000, // 1 minute
  });

  return {
    searchQuery,
    setSearchQuery,
    transactions: transactions || [],
    isLoading,
  };
}

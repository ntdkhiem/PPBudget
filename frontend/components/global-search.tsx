"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Search } from "lucide-react";
import { useDebounce } from "use-debounce";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";

interface SearchResult {
  transactions?: any[];
  categories?: any[];
  accounts?: any[];
  subscriptions?: any[];
}

export function GlobalSearch() {
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const [debouncedQuery] = useDebounce(query, 300);
  const [results, setResults] = React.useState<SearchResult | null>(null);
  const [loading, setLoading] = React.useState(false);
  const router = useRouter();

  React.useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setOpen((open) => !open);
      }
    };

    document.addEventListener("keydown", down);
    return () => document.removeEventListener("keydown", down);
  }, []);

  React.useEffect(() => {
    if (!debouncedQuery) {
      setResults(null);
      return;
    }

    const fetchResults = async () => {
      setLoading(true);
      try {
        const token = localStorage.getItem("ppbudget_token");
        const res = await fetch(`http://localhost:8080/api/v1/search?q=${encodeURIComponent(debouncedQuery)}`, {
          headers: {
            "Authorization": `Bearer ${token}`
          }
        });
        if (res.ok) {
          const data = await res.json();
          setResults(data.results);
        }
      } catch (err) {
        console.error("Failed to fetch search results", err);
      } finally {
        setLoading(false);
      }
    };

    fetchResults();
  }, [debouncedQuery]);

  const onSelect = (href: string) => {
    setOpen(false);
    router.push(href);
  };

  return (
    <>
      <button
        onClick={() => setOpen(true)}
        className="w-full max-w-sm flex items-center gap-2 px-3 py-2 text-sm text-slate-500 bg-slate-100 dark:bg-slate-800/50 dark:bg-slate-800/50 hover:bg-slate-100 dark:hover:bg-slate-800 dark:bg-slate-800 dark:hover:bg-slate-800 border border-slate-200 dark:border-slate-800 rounded-xl transition-colors mb-4"
      >
        <Search className="w-4 h-4" />
        <span className="flex-1 text-left">Search...</span>
        <kbd className="hidden sm:inline-flex h-5 items-center gap-1 rounded border border-slate-200 dark:border-slate-700 bg-slate-100 dark:bg-slate-800 px-1.5 font-mono text-[10px] font-medium text-slate-500 dark:text-slate-400 opacity-100">
          <span className="text-xs">⌘</span>K
        </kbd>
      </button>

      <CommandDialog 
        open={open} 
        onOpenChange={setOpen}
        commandProps={{ shouldFilter: false }}
      >
        <CommandInput 
          placeholder="Type a command or search..." 
          value={query} 
          onValueChange={setQuery} 
        />
        <CommandList>
          {loading && <div className="p-4 text-sm text-slate-500 text-center">Searching...</div>}
          {!loading && results && (
            <CommandEmpty>No results found.</CommandEmpty>
          )}

          {results?.transactions && results.transactions.length > 0 && (
            <CommandGroup heading="Transactions">
              {results.transactions.map((t: any) => (
                <CommandItem key={t.id} onSelect={() => onSelect(`/transactions?edit_id=${t.id}`)}>
                  <span>{t.description.replace(/\s+/g, ' ').trim()}</span>
                  <span className="ml-auto text-slate-500">${(t.amount / 100).toFixed(2)}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          )}


        </CommandList>
      </CommandDialog>
    </>
  );
}

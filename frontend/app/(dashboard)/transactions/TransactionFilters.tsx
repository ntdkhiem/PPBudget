import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { DropdownMenu, DropdownMenuContent, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Search, ListFilter, Wallet, ArrowRightLeft, ChevronDown, XCircle } from "lucide-react";
import { Category, Account } from "@/lib/api";

interface TransactionFiltersProps {
  searchQuery: string;
  setSearchQuery: (q: string) => void;
  selectedCategories: string[];
  toggleCategory: (id: string) => void;
  selectedAccounts: string[];
  toggleAccount: (id: string) => void;
  selectedType: string;
  setSelectedType: (t: string) => void;
  hasActiveFilters: boolean;
  clearFilters: () => void;
  categories: Category[] | undefined;
  accounts: Account[] | undefined;
}

export default function TransactionFilters({
  searchQuery,
  setSearchQuery,
  selectedCategories,
  toggleCategory,
  selectedAccounts,
  toggleAccount,
  selectedType,
  setSelectedType,
  hasActiveFilters,
  clearFilters,
  categories,
  accounts,
}: TransactionFiltersProps) {
  return (
    <div className="w-full relative group mb-6">
      <div className="absolute -inset-0.5 bg-gradient-to-r from-indigo-500/10 to-transparent rounded-xl blur opacity-30 group-hover:opacity-50 transition duration-500"></div>
      
      <div className="relative flex flex-col md:flex-row items-center gap-3 p-2 bg-white/60 dark:bg-slate-900/40 backdrop-blur-xl border border-slate-200/50 dark:border-slate-800/50 shadow-sm rounded-xl transition-all duration-300">
        
        <div className="relative w-full md:w-80 flex-shrink-0">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-slate-400 dark:text-slate-500" />
          <Input
            type="text"
            placeholder="Search by description or amount..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9 pr-4 h-10 w-full bg-transparent border-none shadow-none focus-visible:ring-1 focus-visible:ring-indigo-500/50 text-sm text-slate-900 dark:text-slate-100 placeholder:text-slate-400 dark:placeholder:text-slate-500"
          />
        </div>

        <div className="hidden md:block w-px h-6 bg-slate-200 dark:bg-slate-800 mx-1"></div>

        <div className="flex w-full md:w-auto items-center gap-2 overflow-x-auto pb-1 md:pb-0 scrollbar-hide">
          
          <Popover>
            <PopoverTrigger asChild>
              <Button 
                variant="outline" 
                size="sm"
                className={`h-9 border-slate-200 dark:border-slate-800 transition-all duration-200 ${
                  selectedCategories.length > 0 
                    ? 'bg-indigo-50 dark:bg-indigo-900/20 text-indigo-600 dark:text-indigo-400 border-indigo-200 dark:border-indigo-800 hover:bg-indigo-100 dark:hover:bg-indigo-900/40' 
                    : 'bg-transparent text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800'
                }`}
              >
                <ListFilter className="h-4 w-4 mr-2" />
                Category
                {selectedCategories.length > 0 && (
                  <Badge variant="secondary" className="ml-2 h-5 px-1.5 rounded bg-indigo-100 dark:bg-indigo-900/60 text-indigo-700 dark:text-indigo-300 hover:bg-indigo-200 dark:hover:bg-indigo-800">
                    {selectedCategories.length}
                  </Badge>
                )}
              </Button>
            </PopoverTrigger>
            <PopoverContent align="start" className="w-56 p-2 bg-white/95 dark:bg-slate-900/95 backdrop-blur-md border-slate-200 dark:border-slate-800 rounded-xl">
              <div className="px-2 py-1.5 text-sm font-semibold text-slate-900 dark:text-slate-100">Filter Categories</div>
              <div className="h-px bg-slate-200 dark:bg-slate-800 my-1" />
              <div className="max-h-[300px] overflow-y-auto space-y-0.5">
              {categories?.map(category => (
                <label key={category.id} className="flex items-center gap-2 px-2 py-1.5 text-sm rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 cursor-pointer">
                  <input 
                    type="checkbox" 
                    className="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500 bg-white dark:bg-slate-900 cursor-pointer"
                    checked={selectedCategories.includes(category.id)}
                    onChange={() => toggleCategory(category.id)}
                  />
                  <span className="truncate text-slate-700 dark:text-slate-300">{category.name}</span>
                </label>
              ))}
              </div>
            </PopoverContent>
          </Popover>

          <Popover>
            <PopoverTrigger asChild>
              <Button 
                variant="outline" 
                size="sm"
                className={`h-9 border-slate-200 dark:border-slate-800 transition-all duration-200 ${
                  selectedAccounts.length > 0 
                    ? 'bg-indigo-50 dark:bg-indigo-900/20 text-indigo-600 dark:text-indigo-400 border-indigo-200 dark:border-indigo-800 hover:bg-indigo-100 dark:hover:bg-indigo-900/40' 
                    : 'bg-transparent text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800'
                }`}
              >
                <Wallet className="h-4 w-4 mr-2" />
                Account
                {selectedAccounts.length > 0 && (
                  <Badge variant="secondary" className="ml-2 h-5 px-1.5 rounded bg-indigo-100 dark:bg-indigo-900/60 text-indigo-700 dark:text-indigo-300 hover:bg-indigo-200 dark:hover:bg-indigo-800">
                    {selectedAccounts.length}
                  </Badge>
                )}
              </Button>
            </PopoverTrigger>
            <PopoverContent align="start" className="w-56 p-2 bg-white/95 dark:bg-slate-900/95 backdrop-blur-md border-slate-200 dark:border-slate-800 rounded-xl">
              <div className="px-2 py-1.5 text-sm font-semibold text-slate-900 dark:text-slate-100">Filter Accounts</div>
              <div className="h-px bg-slate-200 dark:bg-slate-800 my-1" />
              <div className="max-h-[300px] overflow-y-auto space-y-0.5">
              {accounts?.map(account => (
                <label key={account.id} className="flex items-center gap-2 px-2 py-1.5 text-sm rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 cursor-pointer">
                  <input 
                    type="checkbox" 
                    className="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500 bg-white dark:bg-slate-900 cursor-pointer"
                    checked={selectedAccounts.includes(account.id)}
                    onChange={() => toggleAccount(account.id)}
                  />
                  <span className="truncate text-slate-700 dark:text-slate-300">{account.name}</span>
                </label>
              ))}
              </div>
            </PopoverContent>
          </Popover>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button 
                variant="outline" 
                size="sm"
                className={`h-9 border-slate-200 dark:border-slate-800 transition-all duration-200 ${
                  selectedType !== 'All' 
                    ? 'bg-indigo-50 dark:bg-indigo-900/20 text-indigo-600 dark:text-indigo-400 border-indigo-200 dark:border-indigo-800 hover:bg-indigo-100 dark:hover:bg-indigo-900/40' 
                    : 'bg-transparent text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800'
                }`}
              >
                <ArrowRightLeft className="h-4 w-4 mr-2" />
                {selectedType === 'All' ? 'Type' : selectedType}
                <ChevronDown className="h-3 w-3 ml-2 opacity-50" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-40 bg-white/95 dark:bg-slate-900/95 backdrop-blur-md border-slate-200 dark:border-slate-800 rounded-xl">
              <DropdownMenuRadioGroup value={selectedType} onValueChange={setSelectedType}>
                {['All', 'Income', 'Expense', 'Transfer'].map(type => (
                  <DropdownMenuRadioItem key={type} value={type} className="cursor-pointer">
                    {type}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>

          <div className="flex-grow"></div>

          {hasActiveFilters && (
            <Button
              variant="ghost"
              size="sm"
              onClick={clearFilters}
              className="h-9 px-3 text-slate-500 hover:text-red-600 hover:bg-red-50 dark:hover:text-red-400 dark:hover:bg-red-900/20 transition-colors"
            >
              <XCircle className="h-4 w-4 mr-2" />
              Clear
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
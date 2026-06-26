"use client";

import { useState, useEffect } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Database, Loader2, CheckCircle2, ChevronRight, ChevronLeft, Calendar as CalendarIcon, Check, RefreshCw } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch, Account } from "@/lib/api";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Calendar } from "@/components/ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Progress } from "@/components/ui/progress";
import { format } from "date-fns";

// Types
interface SimpleFinAccount {
  id: string;
  name: string;
  org: { name: string };
  balance: string;
  currency: string;
}

export default function SimpleFinImporterWizard() {
  const [step, setStep] = useState(0); // 0: loading/dashboard, 1: setup
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";

  // Step 1 State
  const [setupToken, setSetupToken] = useState("");
  const [accessUrl, setAccessUrl] = useState("");

  // Step 2 State
  const [simpleFinAccounts, setSimpleFinAccounts] = useState<SimpleFinAccount[]>([]);
  const [accountMapping, setAccountMapping] = useState<Record<string, string>>({});

  // Step 3 State
  const [importPending, setImportPending] = useState(false);
  const [applyRules, setApplyRules] = useState(true);
  const [contentDedup, setContentDedup] = useState(true);
  const [startDate, setStartDate] = useState<Date | undefined>(() => {
    const d = new Date();
    d.setMonth(d.getMonth() - 1);
    return d;
  });

  // Step 4 State
  const [isExecuting, setIsExecuting] = useState(false);

  // Queries
  const { data: configData, isLoading: configLoading } = useQuery({
    queryKey: ["simplefin-config"],
    queryFn: () => apiFetch<{ connected: boolean; access_token: string; account_mapping?: Record<string, string> }>("/import/simplefin/config", {}, token),
  });

  const { data: localAccounts } = useQuery({
    queryKey: ["accounts"],
    queryFn: () => apiFetch<Account[]>("/accounts", {}, token),
  });

  const { data: statusData } = useQuery({
    queryKey: ["simplefin-status"],
    queryFn: () => apiFetch<{ status: string; current: number; total: number; message: string }>("/import/simplefin/status", {}, token),
    refetchInterval: isExecuting ? 1000 : false,
    enabled: isExecuting,
  });

  useEffect(() => {
    if (configData) {
      if (configData.connected && step === 0) {
        setAccessUrl(configData.access_token);
        // Stay on step 0
      } else if (!configData.connected && step === 0) {
        setStep(1);
      }
    }
  }, [configData, step]);

  useEffect(() => {
    if (statusData?.status === "completed") {
      setIsExecuting(false);
      setStep(5);
    }
  }, [statusData]);

  // Mutations
  const claimMutation = useMutation({
    mutationFn: (setupToken: string) =>
      apiFetch<{ access_url: string }>(
        "/import/simplefin/claim",
        {
          method: "POST",
          body: JSON.stringify({ setup_token: setupToken }),
        },
        token
      ),
  });

  const fetchAccountsMutation = useMutation({
    mutationFn: (url: string) =>
      apiFetch<{ sf_accounts: SimpleFinAccount[] }>(
        "/import/simplefin/fetch-accounts",
        {
          method: "POST",
          body: JSON.stringify({ access_url: url }),
        },
        token
      ),
  });

  const executeMutation = useMutation({
    mutationFn: (data: any) =>
      apiFetch(
        "/import/simplefin/execute",
        {
          method: "POST",
          body: JSON.stringify(data),
        },
        token
      ),
  });

  // Handlers
  const handleSyncNow = async () => {
    if (!accessUrl) return;
    try {
      const accountsRes = await fetchAccountsMutation.mutateAsync(accessUrl);
      setSimpleFinAccounts(accountsRes.sf_accounts || []);
      
      const savedMapping = configData?.account_mapping || {};

      const initMap: Record<string, string> = {};
      (accountsRes.sf_accounts || []).forEach(acc => {
        initMap[acc.id] = savedMapping[acc.id] || "new";
      });
      setAccountMapping(initMap);
      
      setStep(2);
    } catch (err: any) {
      toast.error(err.message || "Failed to fetch accounts.");
    }
  };

  const handleStep1 = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!setupToken) {
      toast.error("Please enter a Setup Token.");
      return;
    }

    try {
      const claimRes = await claimMutation.mutateAsync(setupToken);
      setAccessUrl(claimRes.access_url);
      
      const accountsRes = await fetchAccountsMutation.mutateAsync(claimRes.access_url);
      setSimpleFinAccounts(accountsRes.sf_accounts || []);
      
      const savedMapping = configData?.account_mapping || {};

      const initMap: Record<string, string> = {};
      (accountsRes.sf_accounts || []).forEach(acc => {
        initMap[acc.id] = savedMapping[acc.id] || "new";
      });
      setAccountMapping(initMap);
      
      setStep(2);
      toast.success("Successfully connected to SimpleFin.");
    } catch (err: any) {
      toast.error(err.message || "Failed to connect to SimpleFin.");
    }
  };

  const handleStep2 = (e: React.FormEvent) => {
    e.preventDefault();
    setStep(3);
  };

  const handleStep3 = (e: React.FormEvent) => {
    e.preventDefault();
    if (!startDate) {
      toast.error("Please select a start date.");
      return;
    }
    setStep(4);
  };

  const handleExecute = async () => {
    try {
      await executeMutation.mutateAsync({
        access_url: accessUrl,
        account_mapping: accountMapping,
        start_date: startDate ? startDate.toISOString() : undefined,
        import_pending: importPending,
        apply_rules: applyRules,
        content_dedup: contentDedup,
      });
      setIsExecuting(true);
    } catch (err: any) {
      toast.error(err.message || "Failed to start import.");
    }
  };

  const CustomCheckbox = ({ id, checked, onChange, label, desc }: any) => (
    <label htmlFor={id} className="flex items-start gap-3 cursor-pointer group">
      <div className={`mt-0.5 flex-shrink-0 w-5 h-5 rounded border ${checked ? 'bg-indigo-500 border-indigo-500 text-white' : 'border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900'} flex items-center justify-center transition-colors`}>
        {checked && <Check className="w-3.5 h-3.5" />}
        <input id={id} type="checkbox" className="hidden" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      </div>
      <div>
        <div className="text-sm font-medium text-slate-900 dark:text-slate-100">{label}</div>
        {desc && <div className="text-xs text-slate-500 dark:text-slate-400 mt-1 leading-relaxed">{desc}</div>}
      </div>
    </label>
  );

  return (
    <div className="max-w-3xl mx-auto py-10 px-4">
      <div className="mb-10 text-center">
        <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-gradient-to-br from-indigo-500 to-purple-600 text-white shadow-lg shadow-indigo-500/25 mb-4">
          <Database className="w-8 h-8" />
        </div>
        <h1 className="text-3xl font-bold font-heading text-slate-900 dark:text-white">
          SimpleFin Data Importer
        </h1>
        <p className="text-slate-500 dark:text-slate-400 mt-2 max-w-md mx-auto">
          Connect your accounts and import your financial data seamlessly in 4 easy steps.
        </p>
      </div>

      {step > 0 && step < 5 && (
        <div className="mb-10">
          <div className="flex items-center justify-between mb-2 relative max-w-xl mx-auto">
            <div className="absolute top-1/2 left-0 w-full h-[2px] bg-slate-100 dark:bg-slate-800 -z-10 -translate-y-1/2"></div>
            <div className={`absolute top-1/2 left-0 h-[2px] bg-indigo-500 -z-10 -translate-y-1/2 transition-all duration-500 ease-in-out`} style={{ width: `${((step - 1) / 3) * 100}%` }}></div>
            
            {[1, 2, 3, 4].map((s) => (
              <div key={s} className="flex flex-col items-center">
                <div className={`w-10 h-10 rounded-full flex items-center justify-center transition-all duration-300 ${step >= s ? 'bg-indigo-600 text-white shadow-lg shadow-indigo-500/30' : 'bg-slate-100 dark:bg-slate-800 text-slate-400'}`}>
                  {step > s ? <Check className="w-5 h-5" /> : s}
                </div>
                <div className={`absolute top-12 text-[11px] font-semibold uppercase tracking-wider ${step >= s ? 'text-indigo-600 dark:text-indigo-400' : 'text-slate-400'}`}>
                  {s === 1 ? 'Connect' : s === 2 ? 'Accounts' : s === 3 ? 'Options' : 'Review'}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="bg-white dark:bg-slate-900 rounded-3xl shadow-xl shadow-slate-200/40 dark:shadow-none border border-slate-200 dark:border-slate-800 overflow-hidden relative min-h-[450px]">
        <div className="absolute top-0 left-0 right-0 h-1.5 bg-gradient-to-r from-indigo-500 via-purple-500 to-indigo-500" />
        
        <AnimatePresence mode="wait">
          {step === 0 && (
            <motion.div
              key="step0"
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.95 }}
              className="p-12 flex flex-col items-center justify-center text-center h-[450px]"
            >
              {configLoading ? (
                <div className="flex flex-col items-center">
                  <Loader2 className="w-10 h-10 text-indigo-500 animate-spin mb-4" />
                  <p className="text-slate-500">Checking connection...</p>
                </div>
              ) : configData?.connected ? (
                <>
                  <div className="w-24 h-24 bg-indigo-100 dark:bg-indigo-900/30 text-indigo-600 dark:text-indigo-400 rounded-full flex items-center justify-center mb-6 shadow-xl shadow-indigo-500/10">
                    <Database className="w-12 h-12" />
                  </div>
                  <h2 className="text-3xl font-bold text-slate-900 dark:text-white mb-3">Connected to SimpleFin</h2>
                  <p className="text-slate-500 dark:text-slate-400 max-w-md mb-8 text-lg">
                    Your SimpleFin account is linked and ready to import data. Click the button below to start syncing.
                  </p>
                  <Button 
                    onClick={handleSyncNow} 
                    disabled={fetchAccountsMutation.isPending}
                    className="h-12 px-8 rounded-xl bg-indigo-600 hover:bg-indigo-700 text-white shadow-lg shadow-indigo-500/25 border-0 font-medium text-base transition-all"
                  >
                    {fetchAccountsMutation.isPending ? (
                      <><Loader2 className="w-5 h-5 mr-2 animate-spin" /> Fetching Accounts...</>
                    ) : (
                      <><RefreshCw className="w-5 h-5 mr-2" /> Sync Now</>
                    )}
                  </Button>
                </>
              ) : null}
            </motion.div>
          )}

          {step === 1 && (
            <motion.div
              key="step1"
              initial={{ opacity: 0, x: 20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -20 }}
              className="p-8 sm:p-10"
            >
              <h2 className="text-2xl font-bold mb-6 text-slate-900 dark:text-white">1. Connect to SimpleFin</h2>
              <form onSubmit={handleStep1} className="space-y-6">
                <div>
                  <label className="block text-sm font-semibold text-slate-700 dark:text-slate-300 mb-2">
                    Setup Token
                  </label>
                  <Input
                    autoFocus
                    placeholder="Enter your SimpleFin setup token..."
                    value={setupToken}
                    onChange={(e) => setSetupToken(e.target.value)}
                    className="font-mono bg-slate-50 dark:bg-slate-950/50 border-slate-200 dark:border-slate-800 h-12 text-base rounded-xl"
                  />
                  <p className="text-sm text-slate-500 dark:text-slate-400 mt-3">
                    Generate this one-time setup token from your SimpleFin dashboard. It securely grants read-only access to your transactions.
                  </p>
                </div>

                <div className="flex justify-end pt-4">
                  <Button
                    type="submit"
                    disabled={claimMutation.isPending || fetchAccountsMutation.isPending || !setupToken}
                    className="h-12 px-8 rounded-xl bg-indigo-600 hover:bg-indigo-700 text-white shadow-md font-medium text-base transition-all"
                  >
                    {(claimMutation.isPending || fetchAccountsMutation.isPending) ? (
                      <Loader2 className="w-5 h-5 mr-2 animate-spin" />
                    ) : null}
                    Continue <ChevronRight className="w-5 h-5 ml-1.5 -mr-1" />
                  </Button>
                </div>
              </form>
            </motion.div>
          )}

          {step === 2 && (
            <motion.div
              key="step2"
              initial={{ opacity: 0, x: 20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -20 }}
              className="p-8 sm:p-10 flex flex-col h-[450px]"
            >
              <h2 className="text-2xl font-bold mb-2 text-slate-900 dark:text-white">2. Map Accounts</h2>
              <p className="text-sm text-slate-500 dark:text-slate-400 mb-6">
                We discovered <strong>{simpleFinAccounts.length}</strong> accounts from SimpleFin. Map them to your existing PPBudget accounts or create new ones.
              </p>

              <form onSubmit={handleStep2} className="flex-1 flex flex-col min-h-0">
                <div className="flex-1 overflow-y-auto pr-2 pb-6 space-y-4">
                  {simpleFinAccounts.map((sfAcc) => (
                    <div key={sfAcc.id} className="p-5 rounded-2xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/30 flex flex-col md:flex-row md:items-center justify-between gap-4 hover:border-indigo-200 dark:hover:border-indigo-900/50 transition-colors">
                      <div>
                        <div className="font-semibold text-slate-900 dark:text-slate-100">{sfAcc.name}</div>
                        <div className="text-sm text-slate-500 dark:text-slate-400 mt-1">{sfAcc.org?.name} &middot; {sfAcc.currency} {sfAcc.balance}</div>
                      </div>
                      <div className="w-full md:w-64">
                        <Select
                          value={accountMapping[sfAcc.id] || "new"}
                          onValueChange={(val) => setAccountMapping(prev => ({ ...prev, [sfAcc.id]: val }))}
                        >
                          <SelectTrigger className="bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 h-11 rounded-xl">
                            <SelectValue placeholder="Select mapping..." />
                          </SelectTrigger>
                          <SelectContent className="rounded-xl">
                            <SelectItem value="new" className="font-semibold text-indigo-600 dark:text-indigo-400 py-2.5">
                              + Create New Account
                            </SelectItem>
                            <SelectItem value="skip" className="text-slate-500 py-2.5">
                              Skip importing this account
                            </SelectItem>
                            {localAccounts?.map(la => (
                              <SelectItem key={la.id} value={la.id} className="py-2.5">
                                Map to: {la.name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    </div>
                  ))}
                  {simpleFinAccounts.length === 0 && (
                    <div className="text-center py-10 text-slate-500">
                      No accounts found. This might be an issue with your connection.
                    </div>
                  )}
                </div>

                <div className="flex justify-between items-center pt-6 border-t border-slate-100 dark:border-slate-800 mt-auto">
                  <Button type="button" variant="ghost" onClick={() => setStep(configData?.connected ? 0 : 1)} className="h-12 px-6 rounded-xl text-slate-500 hover:text-slate-700 dark:text-slate-300 dark:hover:text-slate-300">
                    <ChevronLeft className="w-5 h-5 mr-1.5 -ml-1" /> Back
                  </Button>
                  <Button type="submit" className="h-12 px-8 rounded-xl bg-indigo-600 hover:bg-indigo-700 text-white shadow-md font-medium text-base transition-all">
                    Continue <ChevronRight className="w-5 h-5 ml-1.5 -mr-1" />
                  </Button>
                </div>
              </form>
            </motion.div>
          )}

          {step === 3 && (
            <motion.div
              key="step3"
              initial={{ opacity: 0, x: 20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -20 }}
              className="p-8 sm:p-10"
            >
              <h2 className="text-2xl font-bold mb-6 text-slate-900 dark:text-white">3. Import Options</h2>
              <form onSubmit={handleStep3} className="space-y-8">
                
                <div className="space-y-6">
                  <CustomCheckbox
                    id="importPending"
                    checked={importPending}
                    onChange={setImportPending}
                    label="Import Pending Transactions"
                    desc="If checked, we will also import transactions that are still pending. Be aware that pending amounts may change when they clear."
                  />
                  <CustomCheckbox
                    id="applyRules"
                    checked={applyRules}
                    onChange={setApplyRules}
                    label="Apply Categorization Rules"
                    desc="Automatically run your custom rules on all newly imported transactions to assign categories."
                  />
                  <CustomCheckbox
                    id="contentDedup"
                    checked={contentDedup}
                    onChange={setContentDedup}
                    label="Smart De-duplication"
                    desc="Attempt to detect and skip duplicate transactions based on date, amount, and description to prevent double-counting."
                  />
                </div>

                <div className="pt-2">
                  <label className="block text-sm font-semibold text-slate-700 dark:text-slate-300 mb-2">
                    Import Start Date
                  </label>
                  <Popover>
                    <PopoverTrigger asChild>
                      <Button variant="outline" className="w-full md:w-[320px] justify-start text-left font-normal bg-slate-50 dark:bg-slate-950/50 border-slate-200 dark:border-slate-800 h-12 rounded-xl text-base">
                        <CalendarIcon className="mr-3 h-5 w-5 text-slate-400" />
                        {startDate ? format(startDate, "PPP") : <span className="text-slate-400">Pick a date</span>}
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent className="w-auto p-0 rounded-2xl shadow-xl">
                      <Calendar
                        mode="single"
                        selected={startDate}
                        onSelect={(date) => date && setStartDate(date)}
                        className="p-4"
                      />
                    </PopoverContent>
                  </Popover>
                  <p className="text-sm text-slate-500 dark:text-slate-400 mt-3">
                    Transactions before this date will be ignored during the initial import.
                  </p>
                </div>

                <div className="flex justify-between items-center pt-6 border-t border-slate-100 dark:border-slate-800 mt-8">
                  <Button type="button" variant="ghost" onClick={() => setStep(2)} className="h-12 px-6 rounded-xl text-slate-500 hover:text-slate-700 dark:text-slate-300 dark:hover:text-slate-300">
                    <ChevronLeft className="w-5 h-5 mr-1.5 -ml-1" /> Back
                  </Button>
                  <Button type="submit" className="h-12 px-8 rounded-xl bg-indigo-600 hover:bg-indigo-700 text-white shadow-md font-medium text-base transition-all">
                    Review <ChevronRight className="w-5 h-5 ml-1.5 -mr-1" />
                  </Button>
                </div>
              </form>
            </motion.div>
          )}

          {step === 4 && (
            <motion.div
              key="step4"
              initial={{ opacity: 0, x: 20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -20 }}
              className="p-8 sm:p-10"
            >
              <h2 className="text-2xl font-bold mb-6 text-slate-900 dark:text-white">4. Review & Execute</h2>
              
              <div className="bg-slate-50 dark:bg-slate-950/50 rounded-2xl p-8 border border-slate-200 dark:border-slate-800 mb-8 shadow-sm">
                <h3 className="text-lg font-semibold text-slate-900 dark:text-white mb-6 border-b border-slate-200 dark:border-slate-800 pb-3">Import Summary</h3>
                
                <div className="space-y-5">
                  <div className="flex items-center justify-between">
                    <div className="text-slate-500 dark:text-slate-400">Accounts to Import</div>
                    <div className="font-semibold text-slate-900 dark:text-white text-lg">
                      {Object.values(accountMapping).filter(v => v !== 'skip').length}
                    </div>
                  </div>
                  <div className="flex items-center justify-between">
                    <div className="text-slate-500 dark:text-slate-400">Start Date</div>
                    <div className="font-medium text-slate-900 dark:text-white">
                      {startDate ? format(startDate, "PPP") : "N/A"}
                    </div>
                  </div>
                  <div className="flex items-center justify-between">
                    <div className="text-slate-500 dark:text-slate-400">Import Pending</div>
                    <div className="font-medium text-slate-900 dark:text-white">{importPending ? "Yes" : "No"}</div>
                  </div>
                  <div className="flex items-center justify-between">
                    <div className="text-slate-500 dark:text-slate-400">Apply Rules</div>
                    <div className="font-medium text-slate-900 dark:text-white">{applyRules ? "Yes" : "No"}</div>
                  </div>
                </div>
              </div>

              {isExecuting && (
                <div className="mb-8 p-6 bg-slate-50 dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800">
                  <div className="flex justify-between items-center mb-3">
                    <span className="font-medium text-slate-700 dark:text-slate-300">Importing Data...</span>
                    <span className="text-sm text-slate-500">
                      {statusData?.current || 0} / {statusData?.total || 0}
                    </span>
                  </div>
                  <Progress value={statusData?.total ? ((statusData.current / statusData.total) * 100) : 0} className="h-2 mb-2" />
                  <p className="text-sm text-slate-500 text-center">{statusData?.message || "Starting..."}</p>
                </div>
              )}

              <div className="flex justify-between items-center pt-6 border-t border-slate-100 dark:border-slate-800">
                <Button type="button" variant="ghost" onClick={() => setStep(3)} disabled={isExecuting || executeMutation.isPending} className="h-12 px-6 rounded-xl text-slate-500 hover:text-slate-700 dark:text-slate-300 dark:hover:text-slate-300">
                  <ChevronLeft className="w-5 h-5 mr-1.5 -ml-1" /> Back
                </Button>
                <Button 
                  onClick={handleExecute} 
                  disabled={isExecuting || executeMutation.isPending}
                  className="h-12 px-8 rounded-xl bg-gradient-to-r from-emerald-500 to-emerald-600 hover:from-emerald-600 hover:to-emerald-700 text-white shadow-lg shadow-emerald-500/25 border-0 font-medium text-base transition-all"
                >
                  {isExecuting || executeMutation.isPending ? (
                    <><Loader2 className="w-5 h-5 mr-2 animate-spin" /> Executing...</>
                  ) : (
                    <><Database className="w-5 h-5 mr-2" /> Start Import</>
                  )}
                </Button>
              </div>
            </motion.div>
          )}

          {step === 5 && (
            <motion.div
              key="step5"
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              className="p-12 flex flex-col items-center justify-center text-center h-[450px]"
            >
              <div className="w-24 h-24 bg-emerald-100 dark:bg-emerald-900/30 text-emerald-500 rounded-full flex items-center justify-center mb-6 shadow-xl shadow-emerald-500/10">
                <CheckCircle2 className="w-12 h-12" />
              </div>
              <h2 className="text-3xl font-bold text-slate-900 dark:text-white mb-3">Import Completed!</h2>
              <p className="text-slate-500 dark:text-slate-400 max-w-md mb-8 text-lg">
                Your data has been successfully imported and synchronized.
              </p>
              <Button onClick={() => {
                setStep(configData?.connected ? 0 : 1);
                setSimpleFinAccounts([]);
                setAccountMapping({});
              }} variant="outline" className="h-12 px-8 rounded-xl">
                Done
              </Button>
            </motion.div>
          )}

        </AnimatePresence>
      </div>
    </div>
  );
}

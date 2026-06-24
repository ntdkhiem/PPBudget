"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, Suspense } from "react";
import { apiFetch, Rule, Category, RuleCondition, RuleAction } from "@/lib/api";
import { Plus, Trash2, Edit2, ShieldAlert, Zap, Filter, Save, PlusCircle, ArrowRight } from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter, DialogTrigger } from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";

function RulesContent() {
  const queryClient = useQueryClient();
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";

  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<Partial<Rule> | null>(null);

  // Apply to Past State
  const [runOnPast, setRunOnPast] = useState(false);
  const [runAllPast, setRunAllPast] = useState(true);
  const [pastStartDate, setPastStartDate] = useState("");
  const [pastEndDate, setPastEndDate] = useState("");

  // Form State
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [strictness, setStrictness] = useState<"all" | "any">("any");
  const [conditions, setConditions] = useState<Partial<RuleCondition>[]>([{ field: "description", operator: "contains", value: "" }]);
  const [actions, setActions] = useState<Partial<RuleAction>[]>([{ action_type: "set_category", value: "" }]);

  const { data: rules, isLoading: loadingRules } = useQuery<Rule[]>({
    queryKey: ["rules"],
    queryFn: () => apiFetch<Rule[]>("/rules", {}, token),
  });

  const { data: categories } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: () => apiFetch<Category[]>("/categories", {}, token),
  });

  const { data: subscriptions } = useQuery<any[]>({
    queryKey: ["subscriptions"],
    queryFn: () => apiFetch<any[]>("/subscriptions", {}, token),
  });

  const resetForm = () => {
    setName("");
    setDescription("");
    setStrictness("any");
    setConditions([{ field: "description", operator: "contains", value: "" }]);
    setActions([{ action_type: "set_category", value: "" }]);
    setEditingRule(null);
  };

  const openCreate = () => {
    resetForm();
    setRunOnPast(false);
    setIsDialogOpen(true);
  };

  const openEdit = (rule: Rule) => {
    setEditingRule(rule);
    setName(rule.name || "");
    setDescription(rule.description || "");
    setStrictness(rule.strictness || "any");
    setConditions(rule.conditions?.length ? [...rule.conditions] : [{ field: "description", operator: "contains", value: "" }]);
    setActions(rule.actions?.length ? [...rule.actions] : [{ action_type: "set_category", value: "" }]);
    setRunOnPast(false);
    setIsDialogOpen(true);
  };

  const saveMutation = useMutation({
    mutationFn: () => {
      const payload = { name, description, strictness, conditions, actions, is_active: true };
      return apiFetch(editingRule?.id ? `/rules/${editingRule.id}` : "/rules", {
        method: editingRule?.id ? "PUT" : "POST",
        body: JSON.stringify(payload)
      }, token);
    },
    onSuccess: async (data: any) => {
      queryClient.invalidateQueries({ queryKey: ["rules"] });
      setIsDialogOpen(false);
      
      if (runOnPast) {
        try {
          const ruleIdToApply = editingRule?.id || data?.id;
          if (!ruleIdToApply) return;
          
          let sDate = "";
          let eDate = "";
          if (!runAllPast) {
            if (pastStartDate) sDate = new Date(pastStartDate).toISOString();
            if (pastEndDate) eDate = new Date(pastEndDate).toISOString();
          }

          const res = await apiFetch<{ updated_count: number }>(`/rules/${ruleIdToApply}/apply`, {
            method: "POST",
            body: JSON.stringify({
              run_all: runAllPast,
              start_date: sDate,
              end_date: eDate
            })
          }, token);
          alert(`Successfully applied rule to past transactions! Updated ${res.updated_count} transaction(s).`);
        } catch (e) {
          alert("Error applying rule to past transactions.");
        }
      }
      resetForm();
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiFetch(`/rules/${id}`, { method: "DELETE" }, token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["rules"] });
    },
  });

  const addCondition = () => setConditions([...conditions, { field: "description", operator: "contains", value: "" }]);
  const updateCondition = (index: number, field: keyof RuleCondition, val: string) => {
    const newC = [...conditions];
    newC[index] = { ...newC[index], [field]: val };
    setConditions(newC);
  };
  const removeCondition = (index: number) => setConditions(conditions.filter((_, i) => i !== index));

  const addAction = () => setActions([...actions, { action_type: "set_category", value: "" }]);
  const updateAction = (index: number, field: keyof RuleAction, val: string) => {
    const newA = [...actions];
    newA[index] = { ...newA[index], [field]: val };
    setActions(newA);
  };
  const removeAction = (index: number) => setActions(actions.filter((_, i) => i !== index));

  if (loadingRules) return <div className="p-8 text-center text-slate-500">Loading rules...</div>;

  return (
    <div className="max-w-5xl mx-auto space-y-8 pb-12">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-4xl font-extrabold tracking-tight text-slate-900 dark:text-slate-100 mb-2">Rules Engine</h1>
          <p className="text-slate-500 text-lg">Automate your finances with powerful, sleek matching rules.</p>
        </div>
        <Button 
          onClick={openCreate}
          className="bg-indigo-600 hover:bg-indigo-700 text-white shadow-lg hover:shadow-xl transition-all rounded-full px-6 py-6 h-auto flex items-center gap-2"
        >
          <Plus size={20} />
          <span className="font-semibold">Create Rule</span>
        </Button>
      </div>

      <div className="space-y-6 mt-8">
        {rules?.length === 0 ? (
          <div className="text-center py-16 bg-white dark:bg-slate-900/40 backdrop-blur-md rounded-2xl border border-white text-slate-500 shadow-sm">
            <Filter className="mx-auto h-12 w-12 text-slate-300 mb-4" />
            <h3 className="text-lg font-medium text-slate-900 dark:text-slate-100">No rules configured</h3>
            <p className="mt-1">Create one above to get started automating your finances.</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-6">
            <AnimatePresence>
              {rules?.map((rule, idx) => (
                <motion.div
                  key={rule.id}
                  initial={{ opacity: 0, y: 15 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, scale: 0.98 }}
                  transition={{ delay: idx * 0.05 }}
                  className="bg-white dark:bg-slate-900 p-4 rounded-xl border border-slate-200 dark:border-slate-800 shadow-sm hover:shadow-md transition-all group flex items-center justify-between gap-4"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <h3 className="text-base font-bold text-slate-800 dark:text-slate-100 truncate">{rule.name}</h3>
                      {rule.description && <span className="text-xs text-slate-500 truncate">- {rule.description}</span>}
                    </div>

                    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 mt-2">
                      <div className="flex items-center gap-2">
                        <span className="text-[10px] font-bold text-indigo-500 uppercase tracking-wider">If {rule.strictness === 'all' ? 'All' : 'Any'}</span>
                        <div className="flex flex-wrap gap-1">
                          {rule.conditions?.map((c, i) => (
                            <span key={i} className="bg-indigo-50 dark:bg-indigo-500/10 text-indigo-700 dark:text-indigo-300 px-2 py-0.5 rounded text-xs font-medium border border-indigo-100 dark:border-indigo-500/20 whitespace-nowrap">
                              {c.field} {c.operator} "{c.value}"
                            </span>
                          ))}
                        </div>
                      </div>

                      <ArrowRight className="h-4 w-4 text-slate-300 dark:text-slate-600 dark:text-slate-400 hidden sm:block shrink-0" />

                      <div className="flex items-center gap-2">
                        <span className="text-[10px] font-bold text-emerald-500 uppercase tracking-wider">Then</span>
                        <div className="flex flex-wrap gap-1">
                          {rule.actions?.map((a, i) => (
                            <span key={i} className="bg-emerald-50 dark:bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 px-2 py-0.5 rounded text-xs font-medium border border-emerald-100 dark:border-emerald-500/20 whitespace-nowrap">
                              {a.action_type === 'set_category' ? 'Cat' : a.action_type === 'link_to_subscription' ? 'Sub' : a.action_type}: {a.action_type === 'set_category' ? (categories?.find(c => c.id === a.value)?.name || a.value) : a.action_type === 'link_to_subscription' ? (subscriptions?.find((s: any) => s.id === a.value)?.name || a.value) : a.value}
                            </span>
                          ))}
                        </div>
                      </div>
                    </div>
                  </div>
                  
                  <div className="flex gap-1 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                    <Button 
                      variant="ghost" 
                      size="icon"
                      onClick={() => openEdit(rule)}
                      className="h-8 w-8 text-slate-400 hover:text-indigo-600 hover:bg-indigo-50 dark:bg-indigo-900/30 dark:hover:text-indigo-400 dark:hover:bg-indigo-500/10"
                    >
                      <Edit2 size={16} />
                    </Button>
                    <Button 
                      variant="ghost" 
                      size="icon"
                      onClick={() => {
                        if (confirm("Are you sure you want to delete this rule?")) {
                          deleteMutation.mutate(rule.id!);
                        }
                      }}
                      className="h-8 w-8 text-slate-400 hover:text-rose-600 hover:bg-rose-50 dark:hover:text-rose-400 dark:hover:bg-rose-500/10"
                    >
                      <Trash2 size={16} />
                    </Button>
                  </div>
                </motion.div>
              ))}
            </AnimatePresence>
          </div>
        )}
      </div>

      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogContent className="sm:max-w-[90vw] h-[90vh] overflow-y-auto bg-white dark:bg-slate-900/95 backdrop-blur-xl border-slate-200 dark:border-slate-800 shadow-2xl p-0 flex flex-col">
          <div className="p-6 border-b border-slate-100 dark:border-slate-800/50 bg-slate-50 dark:bg-slate-800/50/50">
            <DialogHeader>
              <DialogTitle className="text-2xl font-bold text-slate-800 dark:text-slate-100">
                {editingRule ? "Edit Rule" : "Create New Rule"}
              </DialogTitle>
              <DialogDescription>
                Define triggers and actions to automatically categorize and manage your transactions.
              </DialogDescription>
            </DialogHeader>
          </div>

          <div className="p-6 space-y-8">
            <div className="space-y-4">
              <div className="grid gap-2">
                <Label htmlFor="name" className="text-slate-700 dark:text-slate-300 font-semibold">Rule Name</Label>
                <Input 
                  id="name" 
                  value={name} 
                  onChange={(e) => setName(e.target.value)} 
                  placeholder="e.g. Groceries Auto-Category"
                  className="bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 focus-visible:ring-indigo-500 h-11"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="description" className="text-slate-700 dark:text-slate-300 font-semibold">Description (Optional)</Label>
                <Textarea 
                  id="description" 
                  value={description} 
                  onChange={(e) => setDescription(e.target.value)} 
                  placeholder="What does this rule do?"
                  className="bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 focus-visible:ring-indigo-500 resize-none"
                />
              </div>
            </div>

            <div className="space-y-4">
              <div className="flex items-center justify-between border-b border-slate-100 dark:border-slate-800/50 pb-2">
                <h3 className="text-lg font-bold text-indigo-900 dark:text-indigo-300 flex items-center gap-2">
                  <Filter size={18} className="text-indigo-500"/> Triggers (If)
                </h3>
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-slate-500">Match</span>
                  <Select value={strictness} onValueChange={(val: "all" | "any") => setStrictness(val)}>
                    <SelectTrigger className="w-[120px] h-8 bg-white dark:bg-slate-900">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">All</SelectItem>
                      <SelectItem value="any">Any</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              
              <div className="space-y-3">
                {conditions.map((cond, idx) => (
                  <motion.div 
                    initial={{ opacity: 0, x: -10 }}
                    animate={{ opacity: 1, x: 0 }}
                    key={idx} 
                    className="flex flex-col sm:flex-row gap-3 bg-slate-50 dark:bg-slate-800/50 p-3 rounded-xl border border-slate-200 dark:border-slate-800"
                  >
                    <Select value={cond.field} onValueChange={(v) => updateCondition(idx, "field", v)}>
                      <SelectTrigger className="w-full sm:w-[140px] bg-white dark:bg-slate-900"><SelectValue/></SelectTrigger>
                      <SelectContent>
                        <SelectItem value="description">Description</SelectItem>
                        <SelectItem value="amount">Amount</SelectItem>
                        <SelectItem value="account">Account</SelectItem>
                      </SelectContent>
                    </Select>

                    <Select value={cond.operator} onValueChange={(v) => updateCondition(idx, "operator", v)}>
                      <SelectTrigger className="w-full sm:w-[140px] bg-white dark:bg-slate-900"><SelectValue/></SelectTrigger>
                      <SelectContent>
                        <SelectItem value="contains">Contains</SelectItem>
                        <SelectItem value="is">Is exactly</SelectItem>
                        <SelectItem value="starts_with">Starts with</SelectItem>
                        <SelectItem value="ends_with">Ends with</SelectItem>
                        <SelectItem value="less_than">Less than</SelectItem>
                        <SelectItem value="greater_than">Greater than</SelectItem>
                      </SelectContent>
                    </Select>

                    <Input 
                      value={cond.value} 
                      onChange={(e) => updateCondition(idx, "value", e.target.value)} 
                      placeholder="Value..."
                      className="flex-1 bg-white dark:bg-slate-900"
                    />

                    <Button variant="ghost" size="icon" onClick={() => removeCondition(idx)} disabled={conditions.length === 1} className="text-slate-400 hover:text-red-500">
                      <Trash2 size={16} />
                    </Button>
                  </motion.div>
                ))}
              </div>
              <Button variant="outline" size="sm" onClick={addCondition} className="text-indigo-600 border-indigo-200 bg-indigo-50 dark:bg-indigo-900/30 hover:bg-indigo-100">
                <PlusCircle size={14} className="mr-2" /> Add Condition
              </Button>
            </div>

            <div className="space-y-4">
              <div className="flex items-center justify-between border-b border-slate-100 dark:border-slate-800/50 pb-2">
                <h3 className="text-lg font-bold text-purple-900 dark:text-purple-300 flex items-center gap-2">
                  <Zap size={18} className="text-purple-500"/> Actions (Then)
                </h3>
              </div>
              
              <div className="space-y-3">
                {actions.map((act, idx) => (
                  <motion.div 
                    initial={{ opacity: 0, x: -10 }}
                    animate={{ opacity: 1, x: 0 }}
                    key={idx} 
                    className="flex flex-col sm:flex-row gap-3 bg-purple-50/30 p-3 rounded-xl border border-purple-100"
                  >
                    <Select value={act.action_type} onValueChange={(v) => updateAction(idx, "action_type", v)}>
                      <SelectTrigger className="w-full sm:w-[180px] bg-white dark:bg-slate-900"><SelectValue/></SelectTrigger>
                      <SelectContent>
                        <SelectItem value="set_category">Set Category</SelectItem>
                        <SelectItem value="link_to_subscription">Link to Subscription</SelectItem>
                        <SelectItem value="add_tag">Add Tag</SelectItem>
                        <SelectItem value="set_budget">Set Budget</SelectItem>
                      </SelectContent>
                    </Select>

                    {act.action_type === 'set_category' ? (
                      <Select value={act.value} onValueChange={(v) => updateAction(idx, "value", v)}>
                        <SelectTrigger className="flex-1 bg-white dark:bg-slate-900"><SelectValue placeholder="Select Category..." /></SelectTrigger>
                        <SelectContent>
                          {categories?.map(c => (
                            <SelectItem key={c.id} value={c.id}>{c.name}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    ) : act.action_type === 'link_to_subscription' ? (
                      <Select value={act.value} onValueChange={(v) => updateAction(idx, "value", v)}>
                        <SelectTrigger className="flex-1 bg-white dark:bg-slate-900"><SelectValue placeholder="Select Subscription..." /></SelectTrigger>
                        <SelectContent>
                          {subscriptions?.map(s => (
                            <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    ) : (
                      <Input 
                        value={act.value} 
                        onChange={(e) => updateAction(idx, "value", e.target.value)} 
                        placeholder="Value..."
                        className="flex-1 bg-white dark:bg-slate-900"
                      />
                    )}

                    <Button variant="ghost" size="icon" onClick={() => removeAction(idx)} disabled={actions.length === 1} className="text-slate-400 hover:text-red-500">
                      <Trash2 size={16} />
                    </Button>
                  </motion.div>
                ))}
              </div>
              <Button variant="outline" size="sm" onClick={addAction} className="text-purple-600 border-purple-200 bg-purple-50 hover:bg-purple-100">
                <PlusCircle size={14} className="mr-2" /> Add Action
              </Button>
            </div>
          </div>

            <div className="space-y-4 pt-4 border-t border-slate-100 dark:border-slate-800/50">
              <div className="flex items-center gap-2">
                <input 
                  type="checkbox" 
                  id="runOnPast" 
                  checked={runOnPast} 
                  onChange={(e) => setRunOnPast(e.target.checked)} 
                  className="w-4 h-4 text-indigo-600 rounded border-slate-300 focus:ring-indigo-500"
                />
                <Label htmlFor="runOnPast" className="text-slate-800 dark:text-slate-100 font-semibold cursor-pointer">
                  Run this rule on existing transactions
                </Label>
              </div>
              
              {runOnPast && (
                <motion.div initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: "auto" }} className="pl-6 space-y-4">
                  <div className="flex items-center gap-4">
                    <label className="flex items-center gap-2 text-sm text-slate-700 dark:text-slate-300">
                      <input type="radio" checked={runAllPast} onChange={() => setRunAllPast(true)} />
                      All transactions
                    </label>
                    <label className="flex items-center gap-2 text-sm text-slate-700 dark:text-slate-300">
                      <input type="radio" checked={!runAllPast} onChange={() => setRunAllPast(false)} />
                      Specific date range
                    </label>
                  </div>
                  
                  {!runAllPast && (
                    <div className="flex items-center gap-4">
                      <div className="grid gap-1">
                        <Label className="text-xs text-slate-500">Start Date (Optional)</Label>
                        <Input type="date" value={pastStartDate} onChange={e => setPastStartDate(e.target.value)} className="h-9"/>
                      </div>
                      <div className="grid gap-1">
                        <Label className="text-xs text-slate-500">End Date (Optional)</Label>
                        <Input type="date" value={pastEndDate} onChange={e => setPastEndDate(e.target.value)} className="h-9"/>
                      </div>
                    </div>
                  )}
                </motion.div>
              )}
            </div>

          <div className="p-6 border-t border-slate-100 dark:border-slate-800/50 bg-slate-50 dark:bg-slate-800/50/50 flex justify-end gap-3 sticky bottom-0 mt-auto">
            <Button variant="outline" onClick={() => setIsDialogOpen(false)}>Cancel</Button>
            <Button 
              onClick={() => saveMutation.mutate()} 
              disabled={saveMutation.isPending}
              className="bg-indigo-600 hover:bg-indigo-700 text-white shadow-md"
            >
              <Save size={16} className="mr-2" />
              {saveMutation.isPending ? "Saving..." : "Save Rule"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

export default function RulesPage() {
  return (
    <Suspense fallback={<div className="p-8 text-center text-slate-500">Loading...</div>}>
      <RulesContent />
    </Suspense>
  );
}

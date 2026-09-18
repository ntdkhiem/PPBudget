"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useEffect, useCallback, Suspense } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { toast } from "sonner";
import {
  apiFetch,
  getStoredToken,
  Rule,
  Category,
  Account,
  Subscription,
  RuleCondition,
  RuleAction,
  RuleField,
  RuleOperator,
  RuleActionType,
  RulePreview,
} from "@/lib/api";
import {
  FIELD_LABELS,
  OPERATOR_LABELS,
  FIELD_OPERATORS,
  ACTION_LABELS,
  ACTION_SHORT_LABELS,
  DIRECTION_LABELS,
  MAX_REGEX_LENGTH,
  RULE_FIELDS,
  RULE_ACTION_TYPES,
  isSupportedActionType,
  validateRuleDraft,
} from "@/lib/rules-vocabulary";
import { formatCurrency, formatDate } from "@/lib/utils";
import {
  Plus, Trash2, Edit2, Zap, Filter, Save, PlusCircle, ArrowRight, Check,
  AlertTriangle, Eye, Power, Loader2,
} from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";

/**
 * Condition and action rows carry a stable client id. Using the array index as
 * the React key made removing a middle row reuse the wrong DOM node, so the
 * controlled inputs shifted their values and the enter animation replayed on
 * the wrong element.
 */
type DraftCondition = Partial<RuleCondition> & { key: string };
type DraftAction = Partial<RuleAction> & { key: string };

let keySeq = 0;
const newKey = () => `r${++keySeq}`;

const blankCondition = (): DraftCondition => ({
  key: newKey(),
  field: "description",
  operator: "contains",
  value: "",
});

const blankAction = (): DraftAction => ({
  key: newKey(),
  action_type: "set_category",
  value: "",
});

/** Local date input ("YYYY-MM-DD") to an RFC3339 instant the API accepts. */
function toStartOfDay(value: string): string {
  return value ? new Date(`${value}T00:00:00`).toISOString() : "";
}

/**
 * End dates are inclusive in the UI, so the last day must extend to its final
 * moment. Sending midnight excluded everything that happened on the end date.
 */
function toEndOfDay(value: string): string {
  return value ? new Date(`${value}T23:59:59.999`).toISOString() : "";
}

function RulesContent() {
  const queryClient = useQueryClient();
  const searchParams = useSearchParams();
  const router = useRouter();
  const token = getStoredToken();

  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<Partial<Rule> | null>(null);
  const [ruleToDelete, setRuleToDelete] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  // Apply to past
  const [runOnPast, setRunOnPast] = useState(false);
  const [runAllPast, setRunAllPast] = useState(true);
  const [pastStartDate, setPastStartDate] = useState("");
  const [pastEndDate, setPastEndDate] = useState("");
  const [preview, setPreview] = useState<RulePreview | null>(null);
  const [pendingApply, setPendingApply] = useState<RulePreview | null>(null);

  // Form
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState(0);
  const [strictness, setStrictness] = useState<"all" | "any">("any");
  const [conditions, setConditions] = useState<DraftCondition[]>([blankCondition()]);
  const [actions, setActions] = useState<DraftAction[]>([blankAction()]);

  const { data: rules, isLoading: loadingRules, isError: rulesError } = useQuery<Rule[]>({
    queryKey: ["rules"],
    queryFn: () => apiFetch<Rule[]>("/rules", {}, token),
  });

  const { data: categories } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: () => apiFetch<Category[]>("/categories", {}, token),
  });

  const { data: accounts } = useQuery<Account[]>({
    queryKey: ["accounts"],
    queryFn: () => apiFetch<Account[]>("/accounts", {}, token),
  });

  const { data: subscriptions } = useQuery<Subscription[]>({
    queryKey: ["subscriptions"],
    queryFn: () => apiFetch<Subscription[]>("/subscriptions", {}, token),
  });

  const resetForm = useCallback(() => {
    setName("");
    setDescription("");
    setPriority(0);
    setStrictness("any");
    setConditions([blankCondition()]);
    setActions([blankAction()]);
    setEditingRule(null);
    setFormError(null);
    setPreview(null);
  }, []);

  const openCreate = useCallback(() => {
    resetForm();
    setRunOnPast(false);
    setIsDialogOpen(true);
  }, [resetForm]);

  const openEdit = (rule: Rule) => {
    setEditingRule(rule);
    setName(rule.name || "");
    setDescription(rule.description || "");
    setPriority(rule.priority ?? 0);
    setStrictness(rule.strictness || "any");
    setConditions(
      rule.conditions?.length
        ? rule.conditions.map((c) => ({ ...c, key: newKey() }))
        : [blankCondition()]
    );
    setActions(
      rule.actions?.length
        ? rule.actions.map((a) => ({ ...a, key: newKey() }))
        : [blankAction()]
    );
    setRunOnPast(false);
    setFormError(null);
    setPreview(null);
    setIsDialogOpen(true);
  };

  // "Create rule from this transaction" hands off through query params, the
  // same pattern the transactions page uses for its ?edit= param.
  useEffect(() => {
    if (searchParams.get("new") !== "1") return;

    const prefillDescription = searchParams.get("description") || "";
    const prefillAccount = searchParams.get("account") || "";
    const prefillCategory = searchParams.get("category") || "";

    resetForm();
    setRunOnPast(false);
    setName(prefillDescription ? `Categorize ${prefillDescription.slice(0, 40)}` : "");
    setStrictness("all");
    setConditions([
      { key: newKey(), field: "description", operator: "contains", value: prefillDescription },
      ...(prefillAccount
        ? [{ key: newKey(), field: "account" as RuleField, operator: "is_exactly" as RuleOperator, value: prefillAccount }]
        : []),
    ]);
    setActions([{ key: newKey(), action_type: "set_category", value: prefillCategory }]);
    setIsDialogOpen(true);

    // Strip the params so a refresh does not reopen the dialog.
    router.replace("/settings/rules", { scroll: false });
  }, [searchParams, router, resetForm]);

  const buildPayload = () => ({
    name,
    description,
    strictness,
    priority,
    conditions: conditions.map(({ key, ...c }) => c),
    actions: actions.map(({ key, ...a }) => a),
    is_active: editingRule?.is_active ?? true,
  });

  const applyToPast = async (ruleId: string) => {
    const res = await apiFetch<{ updated_count: number }>(
      `/rules/${ruleId}/apply`,
      {
        method: "POST",
        body: JSON.stringify({
          run_all: runAllPast,
          start_date: runAllPast ? "" : toStartOfDay(pastStartDate),
          end_date: runAllPast ? "" : toEndOfDay(pastEndDate),
        }),
      },
      token
    );
    return res.updated_count;
  };

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload = buildPayload();
      const saved = await apiFetch<Rule>(
        editingRule?.id ? `/rules/${editingRule.id}` : "/rules",
        { method: editingRule?.id ? "PUT" : "POST", body: JSON.stringify(payload) },
        token
      );

      let updatedCount: number | null = null;
      if (runOnPast) {
        const ruleId = editingRule?.id || saved?.id;
        if (ruleId) updatedCount = await applyToPast(ruleId);
      }
      return { updatedCount };
    },
    onSuccess: ({ updatedCount }) => {
      queryClient.invalidateQueries({ queryKey: ["rules"] });
      if (updatedCount !== null) {
        queryClient.invalidateQueries({ queryKey: ["transactions"] });
        toast.success(
          `Rule saved. Updated ${updatedCount} transaction${updatedCount === 1 ? "" : "s"}.`
        );
      } else {
        toast.success("Rule saved.");
      }
      setIsDialogOpen(false);
      setPendingApply(null);
      resetForm();
    },
    onError: (err: Error) => {
      setFormError(err.message);
      toast.error(err.message);
    },
  });

  const previewMutation = useMutation({
    mutationFn: () =>
      apiFetch<RulePreview>(
        "/rules/preview",
        {
          method: "POST",
          body: JSON.stringify({
            ...buildPayload(),
            run_all: runAllPast,
            start_date: runAllPast ? "" : toStartOfDay(pastStartDate),
            end_date: runAllPast ? "" : toEndOfDay(pastEndDate),
            limit: 10,
          }),
        },
        token
      ),
    onSuccess: (data) => {
      setPreview(data);
      setFormError(null);
    },
    onError: (err: Error) => {
      setPreview(null);
      setFormError(err.message);
    },
  });

  const toggleActiveMutation = useMutation({
    mutationFn: (rule: Rule) =>
      apiFetch<Rule>(
        `/rules/${rule.id}`,
        {
          method: "PUT",
          body: JSON.stringify({
            name: rule.name,
            description: rule.description,
            strictness: rule.strictness,
            priority: rule.priority ?? 0,
            conditions: rule.conditions,
            actions: rule.actions,
            is_active: !rule.is_active,
          }),
        },
        token
      ),
    onSuccess: (_data, rule) => {
      queryClient.invalidateQueries({ queryKey: ["rules"] });
      toast.success(`Rule ${rule.is_active ? "disabled" : "enabled"}.`);
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiFetch(`/rules/${id}`, { method: "DELETE" }, token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["rules"] });
      toast.success("Rule deleted.");
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const addCondition = () => setConditions([...conditions, blankCondition()]);
  const removeCondition = (key: string) => setConditions(conditions.filter((c) => c.key !== key));

  const updateCondition = (key: string, patch: Partial<RuleCondition>) => {
    setPreview(null);
    setConditions(conditions.map((c) => (c.key === key ? { ...c, ...patch } : c)));
  };

  /** Changing field resets the operator and value, since neither carries over. */
  const changeConditionField = (key: string, field: RuleField) => {
    const defaultValue = field === "direction" ? "outflow" : "";
    updateCondition(key, { field, operator: FIELD_OPERATORS[field][0], value: defaultValue });
  };

  const addAction = () => setActions([...actions, blankAction()]);
  const removeAction = (key: string) => setActions(actions.filter((a) => a.key !== key));
  const updateAction = (key: string, patch: Partial<RuleAction>) => {
    setPreview(null);
    setActions(actions.map((a) => (a.key === key ? { ...a, ...patch } : a)));
  };

  const handleSave = () => {
    const error = validateRuleDraft(name, conditions, actions);
    if (error) {
      setFormError(error);
      return;
    }
    setFormError(null);

    // A retroactive apply is destructive and has no undo, so confirm against a
    // real count rather than letting it run blind.
    if (runOnPast) {
      if (!runAllPast && !pastStartDate && !pastEndDate) {
        setFormError("Pick a start or end date, or choose 'All past transactions'.");
        return;
      }
      if (preview) {
        setPendingApply(preview);
        return;
      }
      previewMutation.mutate(undefined, {
        onSuccess: (data) => {
          setPreview(data);
          setPendingApply(data);
        },
      });
      return;
    }

    saveMutation.mutate();
  };

  const labelForConditionValue = (c: DraftCondition) => {
    if (c.field === "account") return accounts?.find((a) => a.id === c.value)?.name || c.value || "?";
    if (c.field === "direction") return c.value === "inflow" ? "Inflow" : "Outflow";
    return `"${c.value}"`;
  };

  const labelForActionValue = (a: RuleAction) => {
    switch (a.action_type) {
      case "set_category":
        return categories?.find((c) => c.id === a.value)?.name || a.value;
      case "set_account":
        return accounts?.find((ac) => ac.id === a.value)?.name || a.value;
      case "link_to_subscription":
        return subscriptions?.find((s) => s.id === a.value)?.name || a.value;
      default:
        return a.value;
    }
  };

  const regexError = (c: DraftCondition): string | null => {
    if (c.operator !== "matches_regex" || !c.value) return null;
    if (c.value.length > MAX_REGEX_LENGTH) return `Too long (max ${MAX_REGEX_LENGTH}).`;
    try {
      new RegExp(c.value);
      return null;
    } catch (e) {
      return (e as Error).message;
    }
  };

  if (loadingRules) {
    return (
      <PageContainer maxWidth="5xl">
        <PageHeader title="Rules Engine" description="Automate your finances with matching rules." />
        <div className="space-y-4 mt-8">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-24 w-full rounded-xl" />
          ))}
        </div>
      </PageContainer>
    );
  }

  return (
    <PageContainer maxWidth="5xl">
      <PageHeader
        title="Rules Engine"
        description="Automate your finances with powerful, sleek matching rules."
      >
        <Button
          onClick={openCreate}
          className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl px-5 h-11 shadow-md shadow-indigo-500/20 flex items-center gap-2 transition-all active:scale-95"
        >
          <Plus size={18} /> Add Rule
        </Button>
      </PageHeader>

      <div className="space-y-6 mt-8">
        {rulesError ? (
          <div className="rounded-xl border border-rose-200 dark:border-rose-900/50 bg-rose-50 dark:bg-rose-950/30 p-6 text-center">
            <AlertTriangle className="mx-auto h-8 w-8 text-rose-500 mb-3" />
            <h3 className="font-semibold text-rose-900 dark:text-rose-200">Could not load rules</h3>
            <Button
              variant="outline"
              className="mt-4"
              onClick={() => queryClient.invalidateQueries({ queryKey: ["rules"] })}
            >
              Try again
            </Button>
          </div>
        ) : rules?.length === 0 ? (
          <EmptyState
            icon={Filter}
            title="No rules configured"
            description="Create a rule to automatically categorize transactions as they arrive."
            action={<Button onClick={openCreate}>Create your first rule</Button>}
          />
        ) : (
          <div className="grid grid-cols-1 gap-4">
            <AnimatePresence>
              {rules?.map((rule, idx) => (
                <motion.div
                  key={rule.id}
                  initial={{ opacity: 0, y: 15 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, scale: 0.98 }}
                  transition={{ delay: Math.min(idx * 0.05, 0.3) }}
                  className={`bg-white dark:bg-slate-900 p-4 rounded-xl border border-slate-200 dark:border-slate-800 shadow-sm hover:shadow-md transition-all group flex items-center justify-between gap-4 ${
                    rule.is_active === false ? "opacity-60" : ""
                  }`}
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1 flex-wrap">
                      <h3 className="text-base font-bold text-slate-800 dark:text-slate-100 truncate">
                        {rule.name}
                      </h3>
                      {rule.is_active === false && (
                        <span className="text-[10px] font-bold uppercase tracking-wider px-1.5 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-500">
                          Disabled
                        </span>
                      )}
                      {(rule.priority ?? 0) !== 0 && (
                        <span className="text-[10px] font-semibold px-1.5 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-500">
                          Priority {rule.priority}
                        </span>
                      )}
                      {rule.description && (
                        <span className="text-xs text-slate-500 truncate">- {rule.description}</span>
                      )}
                    </div>

                    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 mt-2">
                      <div className="flex items-center gap-2">
                        <span className="text-[10px] font-bold text-indigo-500 uppercase tracking-wider">
                          If {rule.strictness === "all" ? "All" : "Any"}
                        </span>
                        <div className="flex flex-wrap gap-1">
                          {rule.conditions?.map((c, i) => (
                            <span
                              key={c.id || i}
                              className="bg-indigo-50 dark:bg-indigo-500/10 text-indigo-700 dark:text-indigo-300 px-2 py-0.5 rounded text-xs font-medium border border-indigo-100 dark:border-indigo-500/20 whitespace-nowrap"
                            >
                              {FIELD_LABELS[c.field] ?? c.field}{" "}
                              {(OPERATOR_LABELS[c.operator] ?? c.operator).toLowerCase()}{" "}
                              {labelForConditionValue({ ...c, key: "" })}
                            </span>
                          ))}
                        </div>
                      </div>

                      <ArrowRight className="h-4 w-4 text-slate-300 dark:text-slate-600 hidden sm:block shrink-0" />

                      <div className="flex items-center gap-2">
                        <span className="text-[10px] font-bold text-emerald-500 uppercase tracking-wider">
                          Then
                        </span>
                        <div className="flex flex-wrap gap-1">
                          {rule.actions?.map((a, i) =>
                            isSupportedActionType(a.action_type) ? (
                              <span
                                key={a.id || i}
                                className="bg-emerald-50 dark:bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 px-2 py-0.5 rounded text-xs font-medium border border-emerald-100 dark:border-emerald-500/20 whitespace-nowrap"
                              >
                                {ACTION_SHORT_LABELS[a.action_type]}: {labelForActionValue(a)}
                              </span>
                            ) : (
                              // Left over from action types that were offered in
                              // the UI but never implemented by the engine.
                              <span
                                key={a.id || i}
                                title="This action is no longer supported and does nothing. Edit the rule to replace it."
                                className="bg-amber-50 dark:bg-amber-500/10 text-amber-700 dark:text-amber-300 px-2 py-0.5 rounded text-xs font-medium border border-amber-200 dark:border-amber-500/20 whitespace-nowrap inline-flex items-center gap-1"
                              >
                                <AlertTriangle size={11} /> Unsupported: {a.action_type}
                              </span>
                            )
                          )}
                        </div>
                      </div>
                    </div>
                  </div>

                  <div className="flex gap-1 shrink-0 opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:focus-within:opacity-100 transition-opacity">
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={rule.is_active === false ? "Enable rule" : "Disable rule"}
                      onClick={() => toggleActiveMutation.mutate(rule)}
                      disabled={toggleActiveMutation.isPending}
                      className={`h-8 w-8 ${
                        rule.is_active === false
                          ? "text-slate-400 hover:text-emerald-600 hover:bg-emerald-50 dark:hover:text-emerald-400 dark:hover:bg-emerald-500/10"
                          : "text-emerald-600 dark:text-emerald-400 hover:bg-slate-100 dark:hover:bg-slate-800"
                      }`}
                    >
                      <Power size={16} />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label="Edit rule"
                      onClick={() => openEdit(rule)}
                      className="h-8 w-8 text-slate-400 hover:text-indigo-600 hover:bg-indigo-50 dark:hover:text-indigo-400 dark:hover:bg-indigo-500/10"
                    >
                      <Edit2 size={16} />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label="Delete rule"
                      onClick={() => setRuleToDelete(rule.id!)}
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

      <ConfirmDialog
        open={!!ruleToDelete}
        onOpenChange={(open) => !open && setRuleToDelete(null)}
        title="Delete Rule"
        description="Are you sure you want to delete this rule? This action cannot be undone."
        confirmText="Delete Rule"
        isDestructive
        isLoading={deleteMutation.isPending}
        onConfirm={() => {
          if (ruleToDelete) {
            deleteMutation.mutate(ruleToDelete, { onSuccess: () => setRuleToDelete(null) });
          }
        }}
      />

      <ConfirmDialog
        open={!!pendingApply}
        onOpenChange={(open) => !open && setPendingApply(null)}
        title="Apply rule to existing transactions?"
        description={
          pendingApply
            ? `This will change ${pendingApply.would_update_count} of ${pendingApply.total_scanned} transaction${
                pendingApply.total_scanned === 1 ? "" : "s"
              }. This cannot be undone.`
            : ""
        }
        confirmText={`Update ${pendingApply?.would_update_count ?? 0} transactions`}
        isDestructive
        isLoading={saveMutation.isPending}
        onConfirm={() => saveMutation.mutate()}
      />

      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogContent className="sm:max-w-3xl max-h-[90vh] overflow-y-auto bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-2xl p-0 flex flex-col">
          <div className="p-6 border-b border-slate-100 dark:border-slate-800/50 bg-slate-50 dark:bg-slate-800/50">
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
                <Label htmlFor="name" className="text-slate-700 dark:text-slate-300 font-semibold">
                  Rule Name
                </Label>
                <Input
                  id="name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. Groceries Auto-Category"
                  className="bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 focus-visible:ring-indigo-500 h-11"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="description" className="text-slate-700 dark:text-slate-300 font-semibold">
                  Description (Optional)
                </Label>
                <Textarea
                  id="description"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="What does this rule do?"
                  className="bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 focus-visible:ring-indigo-500 resize-none"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="priority" className="text-slate-700 dark:text-slate-300 font-semibold">
                  Priority
                </Label>
                <Input
                  id="priority"
                  type="number"
                  value={priority}
                  onChange={(e) => setPriority(Number(e.target.value) || 0)}
                  className="bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 h-11 w-32"
                />
                <p className="text-xs text-slate-500">
                  When two rules match the same transaction, the higher priority wins.
                </p>
              </div>
            </div>

            <div className="space-y-4">
              <div className="flex items-center justify-between border-b border-slate-100 dark:border-slate-800/50 pb-2">
                <h3 className="text-lg font-bold text-indigo-900 dark:text-indigo-300 flex items-center gap-2">
                  <Filter size={18} className="text-indigo-500" /> Triggers (If)
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
                {conditions.map((cond) => {
                  const field = (cond.field || "description") as RuleField;
                  const rxError = regexError(cond);
                  return (
                    <motion.div
                      initial={{ opacity: 0, x: -10 }}
                      animate={{ opacity: 1, x: 0 }}
                      key={cond.key}
                      className="bg-slate-50 dark:bg-slate-800/50 p-3 rounded-xl border border-slate-200 dark:border-slate-800"
                    >
                      <div className="flex flex-col sm:flex-row gap-3">
                        <Select
                          value={field}
                          onValueChange={(v) => changeConditionField(cond.key, v as RuleField)}
                        >
                          <SelectTrigger className="w-full sm:w-[140px] bg-white dark:bg-slate-900">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {RULE_FIELDS.map((f) => (
                              <SelectItem key={f} value={f}>
                                {FIELD_LABELS[f]}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>

                        <Select
                          value={cond.operator}
                          onValueChange={(v) => updateCondition(cond.key, { operator: v as RuleOperator })}
                        >
                          <SelectTrigger className="w-full sm:w-[170px] bg-white dark:bg-slate-900">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {FIELD_OPERATORS[field].map((op) => (
                              <SelectItem key={op} value={op}>
                                {OPERATOR_LABELS[op]}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>

                        {field === "account" ? (
                          <Select
                            value={cond.value}
                            onValueChange={(v) => updateCondition(cond.key, { value: v })}
                          >
                            <SelectTrigger className="flex-1 bg-white dark:bg-slate-900">
                              <SelectValue placeholder="Select account..." />
                            </SelectTrigger>
                            <SelectContent>
                              {accounts?.map((a) => (
                                <SelectItem key={a.id} value={a.id}>
                                  {a.name}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        ) : field === "direction" ? (
                          <Select
                            value={cond.value}
                            onValueChange={(v) => updateCondition(cond.key, { value: v })}
                          >
                            <SelectTrigger className="flex-1 bg-white dark:bg-slate-900">
                              <SelectValue placeholder="Select direction..." />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="outflow">{DIRECTION_LABELS.outflow}</SelectItem>
                              <SelectItem value="inflow">{DIRECTION_LABELS.inflow}</SelectItem>
                            </SelectContent>
                          </Select>
                        ) : (
                          <Input
                            value={cond.value}
                            onChange={(e) => updateCondition(cond.key, { value: e.target.value })}
                            placeholder={
                              field === "amount"
                                ? "100.00"
                                : cond.operator === "matches_regex"
                                ? "^AMZN.*"
                                : "Value..."
                            }
                            inputMode={field === "amount" ? "decimal" : undefined}
                            className={`flex-1 bg-white dark:bg-slate-900 ${
                              cond.operator === "matches_regex" ? "font-mono text-sm" : ""
                            } ${rxError ? "border-rose-400 focus-visible:ring-rose-400" : ""}`}
                          />
                        )}

                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label="Remove condition"
                          onClick={() => removeCondition(cond.key)}
                          disabled={conditions.length === 1}
                          className="text-slate-400 hover:text-rose-500 hover:bg-rose-50 dark:hover:bg-rose-500/10"
                        >
                          <Trash2 size={16} />
                        </Button>
                      </div>

                      {field === "amount" && (
                        <p className="text-xs text-slate-500 mt-2">
                          Compares the amount&apos;s size, ignoring sign. Add a Direction condition to
                          match only money out or only money in.
                        </p>
                      )}
                      {rxError && <p className="text-xs text-rose-600 mt-2 font-mono">{rxError}</p>}
                    </motion.div>
                  );
                })}
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={addCondition}
                className="text-indigo-600 border-indigo-200 bg-indigo-50 hover:bg-indigo-100 dark:text-indigo-400 dark:border-indigo-800/50 dark:bg-indigo-900/30 dark:hover:bg-indigo-900/50"
              >
                <PlusCircle size={14} className="mr-2" /> Add Condition
              </Button>
            </div>

            <div className="space-y-4">
              <div className="flex items-center justify-between border-b border-slate-100 dark:border-slate-800/50 pb-2">
                <h3 className="text-lg font-bold text-purple-900 dark:text-purple-300 flex items-center gap-2">
                  <Zap size={18} className="text-purple-500" /> Actions (Then)
                </h3>
              </div>

              <div className="space-y-3">
                {actions.map((act) => (
                  <motion.div
                    initial={{ opacity: 0, x: -10 }}
                    animate={{ opacity: 1, x: 0 }}
                    key={act.key}
                    className="flex flex-col sm:flex-row gap-3 bg-purple-50/30 dark:bg-purple-900/10 p-3 rounded-xl border border-purple-100 dark:border-purple-900/30"
                  >
                    <Select
                      value={isSupportedActionType(act.action_type || "") ? act.action_type : undefined}
                      onValueChange={(v) =>
                        updateAction(act.key, { action_type: v as RuleActionType, value: "" })
                      }
                    >
                      <SelectTrigger className="w-full sm:w-[220px] bg-white dark:bg-slate-900">
                        <SelectValue placeholder="Pick an action..." />
                      </SelectTrigger>
                      <SelectContent>
                        {RULE_ACTION_TYPES.map((t) => (
                          <SelectItem key={t} value={t}>
                            {ACTION_LABELS[t]}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>

                    <Select
                      value={act.value}
                      onValueChange={(v) => updateAction(act.key, { value: v })}
                    >
                      <SelectTrigger className="flex-1 bg-white dark:bg-slate-900">
                        <SelectValue
                          placeholder={
                            act.action_type === "set_account"
                              ? "Select account..."
                              : act.action_type === "link_to_subscription"
                              ? "Select subscription..."
                              : "Select category..."
                          }
                        />
                      </SelectTrigger>
                      <SelectContent>
                        {act.action_type === "set_account"
                          ? accounts?.map((a) => (
                              <SelectItem key={a.id} value={a.id}>
                                {a.name}
                              </SelectItem>
                            ))
                          : act.action_type === "link_to_subscription"
                          ? subscriptions?.map((s) => (
                              <SelectItem key={s.id} value={s.id}>
                                {s.name}
                              </SelectItem>
                            ))
                          : categories?.map((c) => (
                              <SelectItem key={c.id} value={c.id}>
                                {c.name}
                              </SelectItem>
                            ))}
                      </SelectContent>
                    </Select>

                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label="Remove action"
                      onClick={() => removeAction(act.key)}
                      disabled={actions.length === 1}
                      className="text-slate-400 hover:text-rose-500 hover:bg-rose-50 dark:hover:bg-rose-500/10"
                    >
                      <Trash2 size={16} />
                    </Button>
                  </motion.div>
                ))}
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={addAction}
                className="text-purple-600 border-purple-200 bg-purple-50 hover:bg-purple-100 dark:bg-purple-900/30 dark:border-purple-800/50 dark:hover:bg-purple-900/50 dark:text-purple-400"
              >
                <PlusCircle size={14} className="mr-2" /> Add Action
              </Button>
            </div>

            <div className="pt-6 border-t border-slate-100 dark:border-slate-800/50">
              <div
                role="checkbox"
                aria-checked={runOnPast}
                tabIndex={0}
                onClick={() => setRunOnPast(!runOnPast)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    setRunOnPast(!runOnPast);
                  }
                }}
                className={`p-4 rounded-xl border-2 transition-all cursor-pointer flex flex-col sm:flex-row sm:items-center justify-between gap-4 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500 ${
                  runOnPast
                    ? "border-indigo-500 bg-indigo-50/50 dark:bg-indigo-500/10 dark:border-indigo-500/50"
                    : "border-slate-200 dark:border-slate-800 hover:border-indigo-200 dark:hover:border-indigo-800/50 bg-slate-50 dark:bg-slate-800/20"
                }`}
              >
                <div className="flex items-start gap-3">
                  <div
                    className={`mt-0.5 w-5 h-5 rounded-md flex items-center justify-center shrink-0 border transition-colors ${
                      runOnPast
                        ? "bg-indigo-500 border-indigo-500 text-white"
                        : "border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-900"
                    }`}
                  >
                    {runOnPast && <Check size={14} strokeWidth={3} />}
                  </div>
                  <div>
                    <h4 className="font-semibold text-slate-900 dark:text-slate-100">
                      Run rule on existing transactions
                    </h4>
                    <p className="text-sm text-slate-500 dark:text-slate-400 mt-0.5">
                      Apply this rule retroactively. You&apos;ll see how many transactions change
                      before anything is written.
                    </p>
                  </div>
                </div>
              </div>

              <AnimatePresence>
                {runOnPast && (
                  <motion.div
                    initial={{ opacity: 0, height: 0 }}
                    animate={{ opacity: 1, height: "auto" }}
                    exit={{ opacity: 0, height: 0 }}
                    className="overflow-hidden"
                  >
                    <div className="p-4 mt-4 bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-800 space-y-4">
                      <div className="flex flex-col sm:flex-row sm:items-center gap-4">
                        {[
                          { all: true, label: "All past transactions" },
                          { all: false, label: "Specific date range" },
                        ].map((opt) => (
                          <div
                            key={opt.label}
                            role="radio"
                            aria-checked={runAllPast === opt.all}
                            tabIndex={0}
                            onClick={() => {
                              setRunAllPast(opt.all);
                              setPreview(null);
                            }}
                            onKeyDown={(e) => {
                              if (e.key === "Enter" || e.key === " ") {
                                e.preventDefault();
                                setRunAllPast(opt.all);
                                setPreview(null);
                              }
                            }}
                            className={`flex-1 p-3 rounded-lg border cursor-pointer transition-colors flex items-center gap-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500 ${
                              runAllPast === opt.all
                                ? "bg-indigo-50 dark:bg-indigo-500/10 border-indigo-200 dark:border-indigo-500/30"
                                : "border-slate-200 dark:border-slate-800 hover:bg-slate-50 dark:hover:bg-slate-800/50"
                            }`}
                          >
                            <div
                              className={`w-4 h-4 rounded-full border-2 flex items-center justify-center ${
                                runAllPast === opt.all
                                  ? "border-indigo-500"
                                  : "border-slate-300 dark:border-slate-600"
                              }`}
                            >
                              {runAllPast === opt.all && (
                                <div className="w-2 h-2 rounded-full bg-indigo-500" />
                              )}
                            </div>
                            <span className="font-medium text-slate-700 dark:text-slate-300 text-sm">
                              {opt.label}
                            </span>
                          </div>
                        ))}
                      </div>

                      <AnimatePresence>
                        {!runAllPast && (
                          <motion.div
                            initial={{ opacity: 0, height: 0 }}
                            animate={{ opacity: 1, height: "auto" }}
                            exit={{ opacity: 0, height: 0 }}
                            className="flex flex-col sm:flex-row items-center gap-4 pt-2 overflow-hidden"
                          >
                            <div className="grid gap-1.5 w-full">
                              <Label className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
                                Start Date
                              </Label>
                              <Input
                                type="date"
                                value={pastStartDate}
                                onChange={(e) => {
                                  setPastStartDate(e.target.value);
                                  setPreview(null);
                                }}
                                className="bg-slate-50 dark:bg-slate-800/50 border-slate-200 dark:border-slate-800 h-10"
                              />
                            </div>
                            <div className="grid gap-1.5 w-full">
                              <Label className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
                                End Date
                              </Label>
                              <Input
                                type="date"
                                value={pastEndDate}
                                onChange={(e) => {
                                  setPastEndDate(e.target.value);
                                  setPreview(null);
                                }}
                                className="bg-slate-50 dark:bg-slate-800/50 border-slate-200 dark:border-slate-800 h-10"
                              />
                            </div>
                          </motion.div>
                        )}
                      </AnimatePresence>

                      <div className="flex items-center gap-3 pt-2 border-t border-slate-100 dark:border-slate-800">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => previewMutation.mutate()}
                          disabled={previewMutation.isPending}
                          className="gap-2"
                        >
                          {previewMutation.isPending ? (
                            <Loader2 size={14} className="animate-spin" />
                          ) : (
                            <Eye size={14} />
                          )}
                          Preview changes
                        </Button>
                        {preview && (
                          <span className="text-sm text-slate-600 dark:text-slate-300">
                            <strong>{preview.would_update_count}</strong> of {preview.total_scanned}{" "}
                            transactions would change
                            {preview.match_count !== preview.would_update_count && (
                              <span className="text-slate-400">
                                {" "}
                                ({preview.match_count} match, rest already correct)
                              </span>
                            )}
                          </span>
                        )}
                      </div>

                      {preview && preview.samples.length > 0 && (
                        <div className="rounded-lg border border-slate-200 dark:border-slate-800 divide-y divide-slate-100 dark:divide-slate-800 max-h-56 overflow-y-auto">
                          {preview.samples.map((s) => {
                            const before = categories?.find((c) => c.id === s.current_category_id);
                            const after = categories?.find((c) => c.id === s.new_category_id);
                            return (
                              <div
                                key={s.id}
                                className="flex items-center justify-between gap-3 px-3 py-2 text-xs"
                              >
                                <div className="min-w-0 flex-1">
                                  <div className="truncate font-medium text-slate-700 dark:text-slate-200">
                                    {s.description}
                                  </div>
                                  <div className="text-slate-400">{formatDate(s.date)}</div>
                                </div>
                                <div className="shrink-0 tabular-nums text-slate-500">
                                  {formatCurrency(s.amount)}
                                </div>
                                <div className="shrink-0 flex items-center gap-1.5 text-slate-500">
                                  <span className="line-through opacity-60">
                                    {before?.name || "Uncategorized"}
                                  </span>
                                  <ArrowRight size={11} />
                                  <span className="font-semibold text-emerald-600 dark:text-emerald-400">
                                    {after?.name || "Uncategorized"}
                                  </span>
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          </div>

          <div className="p-6 border-t border-slate-100 dark:border-slate-800/50 bg-slate-50 dark:bg-slate-800/50 flex flex-col sm:flex-row sm:justify-end sm:items-center gap-3 sticky bottom-0 mt-auto">
            {formError && (
              <p className="text-sm text-rose-600 dark:text-rose-400 flex-1 flex items-center gap-2">
                <AlertTriangle size={14} className="shrink-0" />
                {formError}
              </p>
            )}
            <div className="flex justify-end gap-3">
              <Button variant="outline" onClick={() => setIsDialogOpen(false)}>
                Cancel
              </Button>
              <Button
                onClick={handleSave}
                disabled={saveMutation.isPending || previewMutation.isPending}
                className="bg-indigo-600 hover:bg-indigo-700 text-white shadow-md"
              >
                <Save size={16} className="mr-2" />
                {saveMutation.isPending ? "Saving..." : "Save Rule"}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </PageContainer>
  );
}

export default function RulesPage() {
  return (
    <Suspense fallback={<div className="p-8 text-center text-slate-500">Loading...</div>}>
      <RulesContent />
    </Suspense>
  );
}

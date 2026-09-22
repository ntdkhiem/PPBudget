"use client";

import { useState } from "react";
import { SlidersHorizontal } from "lucide-react";
import { formatCurrency } from "@/lib/utils";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { PlanningAccount } from "@/lib/api";
import type { Baseline, DebtInfo, FinancialPlan } from "./planning-math";

/** Dollars in the input, cents in the plan. "" means "no override". */
function centsToInput(cents: number | undefined): string {
  return cents == null ? "" : (cents / 100).toFixed(2);
}

function inputToCents(value: string): number | undefined {
  const trimmed = value.trim();
  if (trimmed === "") return undefined;
  const parsed = Number.parseFloat(trimmed);
  if (!Number.isFinite(parsed) || parsed < 0) return undefined;
  return Math.round(parsed * 100);
}

export function PlanSettingsSheet({
  plan,
  baseline,
  accounts,
  onSave,
  saving,
}: {
  plan: FinancialPlan;
  baseline: Baseline;
  accounts: PlanningAccount[];
  onSave: (plan: FinancialPlan) => void;
  saving: boolean;
}) {
  const [open, setOpen] = useState(false);

  const [income, setIncome] = useState("");
  const [essentials, setEssentials] = useState("");
  const [targetMonths, setTargetMonths] = useState("6");
  const [investApr, setInvestApr] = useState("0");
  const [liquidIds, setLiquidIds] = useState<string[]>([]);
  // APRs are held as raw input strings so reformatting never fights typing;
  // they are converted to rates only on save.
  const [aprInputs, setAprInputs] = useState<Record<string, string>>({});

  /**
   * Seed the form from the saved plan when the sheet opens, so a cancelled
   * edit never leaks into the next one. Done here rather than in an effect:
   * opening is an event, not state to synchronize.
   */
  const handleOpenChange = (next: boolean) => {
    if (next) {
      setIncome(centsToInput(plan.overrides.monthly_income_cents));
      setEssentials(centsToInput(plan.overrides.essential_expenses_cents));
      setTargetMonths(String(plan.emergency_fund.target_months));
      setInvestApr((plan.assumptions.invest_return_apr * 100).toFixed(1));
      setLiquidIds(plan.overrides.liquid_account_ids ?? []);
      setAprInputs(
        Object.fromEntries(plan.debts.map((d) => [d.account_id, (d.apr * 100).toFixed(1)])),
      );
    }
    setOpen(next);
  };

  const assetAccounts = accounts.filter((a) => a.type === "asset");
  const liabilityAccounts = accounts.filter((a) => a.type === "liability");

  const toggleLiquid = (id: string) => {
    setLiquidIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  };

  const handleSave = () => {
    const months = Number.parseInt(targetMonths, 10);
    const apr = Number.parseFloat(investApr);

    const debts: DebtInfo[] = Object.entries(aprInputs).flatMap(([accountId, raw]) => {
      const parsed = Number.parseFloat(raw);
      if (!Number.isFinite(parsed) || parsed <= 0) return [];
      return [
        {
          account_id: accountId,
          apr: Math.min(Math.max(parsed / 100, 0), 1),
          min_payment_cents:
            plan.debts.find((d) => d.account_id === accountId)?.min_payment_cents ?? 0,
        },
      ];
    });

    onSave({
      ...plan,
      overrides: {
        monthly_income_cents: inputToCents(income),
        essential_expenses_cents: inputToCents(essentials),
        liquid_account_ids: liquidIds.length > 0 ? liquidIds : undefined,
      },
      emergency_fund: {
        target_months: Number.isFinite(months) ? Math.min(Math.max(months, 1), 24) : 6,
      },
      assumptions: {
        invest_return_apr: Number.isFinite(apr) ? Math.min(Math.max(apr / 100, 0), 0.5) : 0,
      },
      debts,
    });
    setOpen(false);
  };

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetTrigger asChild>
        <Button variant="outline" className="text-slate-700 dark:text-slate-200">
          <SlidersHorizontal className="w-4 h-4 mr-2" />
          Plan settings
        </Button>
      </SheetTrigger>

      <SheetContent className="w-full sm:max-w-md overflow-y-auto">
        <SheetHeader>
          <SheetTitle>Plan settings</SheetTitle>
          <SheetDescription>
            Every figure is computed from your data. Override one only when the computed value is
            wrong — leave a field empty to keep the automatic number.
          </SheetDescription>
        </SheetHeader>

        <div className="space-y-6 px-4 pb-4">
          <div className="space-y-2">
            <Label htmlFor="plan-income">Monthly income</Label>
            <Input
              id="plan-income"
              inputMode="decimal"
              placeholder={`Auto: ${formatCurrency(baseline.monthlyIncome)}`}
              value={income}
              onChange={(e) => setIncome(e.target.value)}
            />
            <p className="text-xs text-slate-500 dark:text-slate-400">
              Use this when a bonus or a reimbursement inflated the computed average.
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="plan-essentials">Essential monthly expenses</Label>
            <Input
              id="plan-essentials"
              inputMode="decimal"
              placeholder={`Auto: ${formatCurrency(baseline.essentialMonthly)}`}
              value={essentials}
              onChange={(e) => setEssentials(e.target.value)}
            />
            <p className="text-xs text-slate-500 dark:text-slate-400">
              Computed from trailing spend in categories budgeted as &ldquo;needs&rdquo;. This is the
              denominator for every emergency-fund number on the page.
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="plan-target-months">Emergency fund target (months)</Label>
            <Input
              id="plan-target-months"
              inputMode="numeric"
              value={targetMonths}
              onChange={(e) => setTargetMonths(e.target.value)}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="plan-invest-apr">Assumed investment return (% per year)</Label>
            <Input
              id="plan-invest-apr"
              inputMode="decimal"
              value={investApr}
              onChange={(e) => setInvestApr(e.target.value)}
            />
            <p className="text-xs text-slate-500 dark:text-slate-400">
              Used only by the projection. Leave at 0 to project cash with no growth.
            </p>
          </div>

          <div className="space-y-2">
            <Label>Accounts that count as your emergency fund</Label>
            {assetAccounts.length === 0 ? (
              <p className="text-xs text-slate-500 dark:text-slate-400">No asset accounts found.</p>
            ) : (
              <>
                <div className="space-y-1.5">
                  {assetAccounts.map((account) => (
                    <label
                      key={account.id}
                      className="flex items-center gap-3 rounded-lg border border-slate-200 p-2.5 text-sm dark:border-slate-800"
                    >
                      <input
                        type="checkbox"
                        className="h-4 w-4 rounded border-slate-300 accent-indigo-600"
                        checked={liquidIds.includes(account.id)}
                        onChange={() => toggleLiquid(account.id)}
                      />
                      <span className="flex-1 truncate text-slate-900 dark:text-white">
                        {account.name}
                      </span>
                      <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
                        {formatCurrency(account.balance)}
                      </span>
                    </label>
                  ))}
                </div>
                <p className="text-xs text-slate-500 dark:text-slate-400">
                  Select none to count every asset account. Select some to exclude illiquid ones
                  like retirement or property.
                </p>
              </>
            )}
          </div>

          <div className="space-y-2">
            <Label>Interest rates on debt (% APR)</Label>
            {liabilityAccounts.length === 0 ? (
              <p className="text-xs text-slate-500 dark:text-slate-400">
                No liability accounts found.
              </p>
            ) : (
              <>
                <div className="space-y-1.5">
                  {liabilityAccounts.map((account) => (
                      <div
                        key={account.id}
                        className="flex items-center gap-3 rounded-lg border border-slate-200 p-2.5 text-sm dark:border-slate-800"
                      >
                        <span className="flex-1 truncate text-slate-900 dark:text-white">
                          {account.name}
                          <span className="block text-xs font-medium text-slate-500 dark:text-slate-400">
                            {formatCurrency(Math.abs(account.balance))} owed
                          </span>
                        </span>
                        <Input
                          className="w-20"
                          inputMode="decimal"
                          placeholder="—"
                          value={aprInputs[account.id] ?? ""}
                          onChange={(e) =>
                            setAprInputs((prev) => ({ ...prev, [account.id]: e.target.value }))
                          }
                          aria-label={`APR for ${account.name}`}
                        />
                      </div>
                  ))}
                </div>
                <p className="text-xs text-slate-500 dark:text-slate-400">
                  PPBudget does not store interest rates, so the waterfall cannot order debt payoff
                  until you enter them here.
                </p>
              </>
            )}
          </div>
        </div>

        <SheetFooter>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? "Saving…" : "Save plan"}
          </Button>
          <Button
            variant="outline"
            className="text-slate-700 dark:text-slate-200"
            onClick={() => handleOpenChange(false)}
          >
            Cancel
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

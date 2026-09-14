const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

export function getStoredToken(): string {
  return typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";
}

export async function apiFetch<T>(
  endpoint: string,
  options: RequestInit = {},
  token?: string
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...options.headers as Record<string, string>,
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE_URL}${endpoint}`, {
    ...options,
    headers,
    cache: "no-store", // Always fetch fresh financial data
  });

  if (!res.ok) {
    if (res.status === 401 && typeof window !== "undefined" && !endpoint.startsWith("/auth/")) {
      localStorage.removeItem("ppbudget_token");
      window.location.href = "/login";
    }
    const errorBody = await res.json().catch(() => ({}));
    throw new Error(errorBody.error || "API request failed");
  }
  return res.json();
}

// Types matching the Go backend
export interface Account {
  id: string;
  user_id?: string;
  name: string;
  type: "asset" | "liability" | "income" | "expense" | "equity";
  currency?: string;
  current_balance: number; // cents, signed (liabilities negative)
  balance_as_of?: string; // "YYYY-MM-DDT00:00:00Z" — latest non-opening snapshot
  balance_source?: "simplefin" | "manual";
  balance_only: boolean;
  simplefin_id?: string;
  created_at?: string;
  updated_at?: string;
}

export interface BalanceSnapshot {
  id: string;
  account_id: string;
  as_of_date: string | null; // null = opening
  balance: number; // cents
  available_balance?: number;
  source: "opening" | "simplefin" | "manual";
  reported_at?: string;
  created_at: string;
}

export interface Transaction {
  id: string;
  account_id: string;
  amount: number; // cents
  date: string;
  description: string;
  notes?: string | null;
  is_reviewed: boolean;
  running_balance?: number; // cents
  category_id?: string | null;
  effective_amount?: number;
  pays_for?: { transaction_id: string; amount: number }[];
  paid_by?: { transaction_id: string; amount: number }[];
  category?: { id: string; name: string };
  subscription_id?: string;
}

export interface Category {
  id: string;
  name: string;
  type: "income" | "expense" | "transfer";
  transaction_count?: number;
}

export interface RuleCondition {
  id?: string;
  rule_id?: string;
  field: string;
  operator: string;
  value: string;
}

export interface RuleAction {
  id?: string;
  rule_id?: string;
  action_type: string;
  value: string;
}

export interface Rule {
  id?: string;
  name: string;
  description?: string;
  trigger_type?: string;
  priority?: number;
  is_active?: boolean;
  strictness?: "all" | "any";
  conditions: RuleCondition[];
  actions: RuleAction[];
  created_at?: string;
  updated_at?: string;
}

export interface Budget {
  id: string;
  name: string;
  category_id: string;
  amount_cents: number;
  period_type: string;
  start_date: string;
  end_date: string;
  bucket?: "needs" | "wants" | "savings";
}

export interface BudgetSummary extends Budget {
  spent_total: number;
  spent_per_day: number;
  left_total: number;
  left_per_day: number;
}

export interface Subscription {
  id: string;
  name: string;
  amount: number; // cents
  billing_cycle: "weekly" | "monthly" | "yearly";
  next_billing_date: string;
  category_id?: string | null;
}

export interface NetWorthDataPoint {
  month: string;
  assets: number;
  liabilities: number;
  net_worth: number;
}

export interface CategorySpend {
  category_id: string | null; // null for uncategorized
  name: string;
  total_spent: number; // cents
}

export interface DashboardSummary {
  in_period: number;
  out_period: number;
  subscriptions_to_pay: number;
  subscriptions_paid: number;
  left_to_spend: number;
  net_worth: number;
}


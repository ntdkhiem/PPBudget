const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

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
    const errorBody = await res.json().catch(() => ({}));
    throw new Error(errorBody.error || "API request failed");
  }
  return res.json();
}

// Types matching the Go backend
export interface Account {
  id: string;
  name: string;
  type: "asset" | "liability" | "income" | "expense" | "equity";
  initial_balance: number; // cents
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
  category?: { id: string; name: string };
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
  billing_cycle: "monthly" | "yearly";
  next_billing_date: string;
  category_id?: string | null;
}

export interface NetWorthDataPoint {
  date: string;
  value: number; // cents
}

export interface SpendingDataPoint {
  category: string;
  value: number; // cents
}

export interface DashboardSummary {
  in_period: number;
  out_period: number;
  subscriptions_to_pay: number;
  subscriptions_paid: number;
  left_to_spend: number;
  net_worth: number;
}

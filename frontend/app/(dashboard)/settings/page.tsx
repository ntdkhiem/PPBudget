"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { motion } from "framer-motion";
import { apiFetch } from "@/lib/api";
import {
  Key,
  Copy,
  Check,
  RefreshCw,
  SlidersHorizontal,
  Database,
  Tag,
  ShieldCheck,
  Terminal,
  ExternalLink,
  Eye,
  EyeOff,
  Sparkles,
  Smartphone,
  ArrowRight,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";

export default function SettingsPage() {
  const [token, setToken] = useState<string>("");
  const [apiToken, setApiToken] = useState<string>("");
  const [loadingToken, setLoadingToken] = useState<boolean>(true);
  const [generating, setGenerating] = useState<boolean>(false);
  const [showToken, setShowToken] = useState<boolean>(false);
  const [copiedToken, setCopiedToken] = useState<boolean>(false);
  const [copiedPayload, setCopiedPayload] = useState<boolean>(false);

  useEffect(() => {
    const jwt = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";
    setToken(jwt);

    if (jwt) {
      fetchApiToken(jwt);
    } else {
      setLoadingToken(false);
    }
  }, []);

  const fetchApiToken = async (jwtToken: string) => {
    try {
      setLoadingToken(true);
      const res = await apiFetch<{ token: string }>("/settings/token", {}, jwtToken);
      setApiToken(res.token || "");
    } catch {
      // If token not found yet or error, keep blank
      setApiToken("");
    } finally {
      setLoadingToken(false);
    }
  };

  const handleGenerateToken = async () => {
    try {
      setGenerating(true);
      const res = await apiFetch<{ token: string; message: string }>(
        "/settings/generate-token",
        { method: "POST" },
        token
      );
      setApiToken(res.token);
      setShowToken(true);
      toast.success("New Personal API Token generated!");
    } catch (err: any) {
      toast.error(err.message || "Failed to generate API token");
    } finally {
      setGenerating(false);
    }
  };

  const copyToClipboard = (text: string, isPayload = false) => {
    if (!text) return;
    navigator.clipboard.writeText(text);
    if (isPayload) {
      setCopiedPayload(true);
      setTimeout(() => setCopiedPayload(false), 2000);
      toast.success("Webhook payload copied to clipboard");
    } else {
      setCopiedToken(true);
      setTimeout(() => setCopiedToken(false), 2000);
      toast.success("API token copied to clipboard");
    }
  };

  const samplePayload = JSON.stringify(
    {
      simplefin_account_id: "acc_12345",
      simplefin_transaction_id: "txn_67890",
      amount: "-45.50",
      date: new Date().toISOString().split("T")[0],
      description: "Coffee Shop",
    },
    null,
    2
  );

  return (
    <div className="flex-1 p-6 md:p-8 max-w-6xl mx-auto space-y-8">
      {/* Header */}
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-indigo-50 dark:bg-indigo-500/10 text-indigo-600 dark:text-indigo-400">
            <ShieldCheck className="h-6 w-6" />
          </div>
          <div>
            <h1 className="text-3xl font-bold text-slate-900 dark:text-white font-heading">
              Settings & Preferences
            </h1>
            <p className="text-sm text-slate-500 dark:text-slate-400">
              Manage your personal tenant security, webhooks, automation rules, and data connections.
            </p>
          </div>
        </div>
      </div>

      {/* Quick Navigation Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
        <Link
          href="/settings/importer"
          className="group p-5 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 hover:border-indigo-500 dark:hover:border-indigo-500/50 rounded-2xl shadow-sm transition-all duration-200 flex flex-col justify-between hover:shadow-md"
        >
          <div className="space-y-3">
            <div className="w-10 h-10 rounded-xl bg-blue-50 dark:bg-blue-500/10 text-blue-600 dark:text-blue-400 flex items-center justify-center">
              <Database className="h-5 w-5" />
            </div>
            <h3 className="font-semibold text-slate-900 dark:text-white text-base group-hover:text-indigo-600 dark:group-hover:text-indigo-400 transition-colors">
              SimpleFin Importer
            </h3>
            <p className="text-xs text-slate-500 dark:text-slate-400 line-clamp-2 leading-relaxed">
              Connect your financial institutions, map accounts, and toggle scheduled auto-sync.
            </p>
          </div>
          <div className="mt-4 pt-3 border-t border-slate-100 dark:border-slate-800/80 flex items-center text-xs font-medium text-indigo-600 dark:text-indigo-400 gap-1">
            <span>Configure Importer</span>
            <ArrowRight className="h-3.5 w-3.5 group-hover:translate-x-1 transition-transform" />
          </div>
        </Link>

        <Link
          href="/settings/categories"
          className="group p-5 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 hover:border-indigo-500 dark:hover:border-indigo-500/50 rounded-2xl shadow-sm transition-all duration-200 flex flex-col justify-between hover:shadow-md"
        >
          <div className="space-y-3">
            <div className="w-10 h-10 rounded-xl bg-emerald-50 dark:bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
              <Tag className="h-5 w-5" />
            </div>
            <h3 className="font-semibold text-slate-900 dark:text-white text-base group-hover:text-indigo-600 dark:group-hover:text-indigo-400 transition-colors">
              Categories
            </h3>
            <p className="text-xs text-slate-500 dark:text-slate-400 line-clamp-2 leading-relaxed">
              Manage income, expense, and transfer categories to classify your transactions.
            </p>
          </div>
          <div className="mt-4 pt-3 border-t border-slate-100 dark:border-slate-800/80 flex items-center text-xs font-medium text-indigo-600 dark:text-indigo-400 gap-1">
            <span>Manage Categories</span>
            <ArrowRight className="h-3.5 w-3.5 group-hover:translate-x-1 transition-transform" />
          </div>
        </Link>

        <Link
          href="/settings/rules"
          className="group p-5 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 hover:border-indigo-500 dark:hover:border-indigo-500/50 rounded-2xl shadow-sm transition-all duration-200 flex flex-col justify-between hover:shadow-md"
        >
          <div className="space-y-3">
            <div className="w-10 h-10 rounded-xl bg-purple-50 dark:bg-purple-500/10 text-purple-600 dark:text-purple-400 flex items-center justify-center">
              <SlidersHorizontal className="h-5 w-5" />
            </div>
            <h3 className="font-semibold text-slate-900 dark:text-white text-base group-hover:text-indigo-600 dark:group-hover:text-indigo-400 transition-colors">
              Automation Rules
            </h3>
            <p className="text-xs text-slate-500 dark:text-slate-400 line-clamp-2 leading-relaxed">
              Build rules to auto-categorize incoming transactions and link subscriptions.
            </p>
          </div>
          <div className="mt-4 pt-3 border-t border-slate-100 dark:border-slate-800/80 flex items-center text-xs font-medium text-indigo-600 dark:text-indigo-400 gap-1">
            <span>Configure Rules</span>
            <ArrowRight className="h-3.5 w-3.5 group-hover:translate-x-1 transition-transform" />
          </div>
        </Link>
      </div>

      {/* Personal API Key & Ingest Webhook Section */}
      <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-sm overflow-hidden">
        <div className="p-6 border-b border-slate-100 dark:border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-amber-50 dark:bg-amber-500/10 text-amber-600 dark:text-amber-400 rounded-xl">
              <Key className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-lg font-bold text-slate-900 dark:text-white">
                Personal API Token & Ingest Webhook
              </h2>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                Use your personal API token to securely ingest transactions from Apple Shortcuts, Siri, or external webhooks.
              </p>
            </div>
          </div>
          <Button
            onClick={handleGenerateToken}
            disabled={generating}
            className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl flex items-center gap-2 text-xs font-medium cursor-pointer"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${generating ? "animate-spin" : ""}`} />
            <span>{apiToken ? "Regenerate Token" : "Generate Token"}</span>
          </Button>
        </div>

        <div className="p-6 space-y-6">
          {/* Token Display Field */}
          <div className="space-y-2">
            <label className="text-xs font-semibold uppercase tracking-wider text-slate-600 dark:text-slate-400 flex items-center gap-1.5">
              <span>Your Personal API Key</span>
              <span className="text-[10px] px-2 py-0.5 rounded-full bg-emerald-50 dark:bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-normal">
                User-Scoped
              </span>
            </label>

            {loadingToken ? (
              <div className="h-11 bg-slate-100 dark:bg-slate-800 rounded-xl animate-pulse" />
            ) : apiToken ? (
              <div className="flex items-center gap-2">
                <div className="relative flex-1">
                  <input
                    type={showToken ? "text" : "password"}
                    readOnly
                    value={apiToken}
                    className="w-full pl-4 pr-10 py-2.5 bg-slate-50 dark:bg-slate-800/80 border border-slate-200 dark:border-slate-700 rounded-xl text-slate-900 dark:text-white font-mono text-sm focus:outline-none select-all"
                  />
                  <button
                    type="button"
                    onClick={() => setShowToken(!showToken)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors"
                  >
                    {showToken ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => copyToClipboard(apiToken)}
                  className="rounded-xl flex items-center gap-1.5 text-xs h-10 px-3 cursor-pointer"
                >
                  {copiedToken ? (
                    <>
                      <Check className="h-3.5 w-3.5 text-emerald-600" />
                      <span>Copied</span>
                    </>
                  ) : (
                    <>
                      <Copy className="h-3.5 w-3.5" />
                      <span>Copy</span>
                    </>
                  )}
                </Button>
              </div>
            ) : (
              <div className="p-4 bg-slate-50 dark:bg-slate-800/50 border border-dashed border-slate-200 dark:border-slate-700 rounded-xl text-center">
                <p className="text-xs text-slate-500 dark:text-slate-400 mb-2">
                  No personal API token generated yet. Click &ldquo;Generate Token&rdquo; above to create one.
                </p>
              </div>
            )}
          </div>

          {/* Webhook / Apple Shortcuts Instructions */}
          <div className="p-5 bg-slate-50 dark:bg-slate-950/60 rounded-xl border border-slate-200 dark:border-slate-800 space-y-4">
            <div className="flex items-center gap-2 text-slate-900 dark:text-white font-semibold text-sm">
              <Smartphone className="h-4 w-4 text-indigo-500" />
              <span>How to Ingest Transactions via Machine-to-Machine Webhook</span>
            </div>

            <p className="text-xs text-slate-600 dark:text-slate-400 leading-relaxed">
              To send new transactions automatically (e.g. from an Apple Shortcut after Apple Pay, or a custom bank webhook script), make an HTTP <code className="px-1.5 py-0.5 bg-slate-200 dark:bg-slate-800 rounded font-mono text-indigo-600 dark:text-indigo-400">POST</code> request to the ingestion endpoint:
            </p>

            <div className="space-y-2">
              <div className="flex items-center justify-between text-xs font-mono text-slate-500">
                <span>cURL / Webhook Example</span>
                <button
                  onClick={() => copyToClipboard(samplePayload, true)}
                  className="flex items-center gap-1 text-indigo-600 dark:text-indigo-400 hover:underline cursor-pointer"
                >
                  {copiedPayload ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
                  <span>Copy Payload</span>
                </button>
              </div>
              <pre className="p-3.5 bg-slate-900 text-slate-100 rounded-xl text-xs font-mono overflow-x-auto border border-slate-800 leading-relaxed">
{`curl -X POST "${typeof window !== "undefined" ? window.location.origin : "http://localhost:8080"}/api/v1/ingest" \\
  -H "Content-Type: application/json" \\
  -H "X-API-Key: ${apiToken || "<YOUR_PERSONAL_API_TOKEN>"}" \\
  -d '${samplePayload}'`}
              </pre>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2 text-xs text-slate-500 dark:text-slate-400">
              <div className="flex items-start gap-2">
                <Sparkles className="h-4 w-4 text-indigo-500 shrink-0 mt-0.5" />
                <span>Auto-categorization rules and subscription matching will run automatically on incoming transactions.</span>
              </div>
              <div className="flex items-start gap-2">
                <ShieldCheck className="h-4 w-4 text-emerald-500 shrink-0 mt-0.5" />
                <span>Transactions are automatically isolated and saved directly into your personal user account.</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

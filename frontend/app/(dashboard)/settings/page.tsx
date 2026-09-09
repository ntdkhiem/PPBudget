"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { apiFetch } from "@/lib/api";
import {
  ShieldCheck,
  Lock,
  Database,
  Download,
  AlertTriangle,
  Mail,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export default function SettingsPage() {
  const router = useRouter();
  const [token, setToken] = useState<string>("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [isChangingPassword, setIsChangingPassword] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isTestingEmail, setIsTestingEmail] = useState(false);
  const [exportTypes, setExportTypes] = useState({
    accounts: true,
    categories: true,
    transactions: true,
    budgets: true,
    rules: true,
    subscriptions: true,
  });

  const handleTestEmail = async () => {
    try {
      setIsTestingEmail(true);
      await apiFetch("/settings/test-email", { method: "POST" }, token);
      toast.success("Test email sent successfully! Please check your inbox.");
    } catch (err: any) {
      toast.error(err.message || "Failed to send test email");
    } finally {
      setIsTestingEmail(false);
    }
  };

  useEffect(() => {
    const jwt = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";
    setToken(jwt);
  }, []);

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!currentPassword || !newPassword) {
      toast.error("Please fill in both password fields");
      return;
    }

    try {
      setIsChangingPassword(true);
      await apiFetch(
        "/settings/password",
        {
          method: "POST",
          body: JSON.stringify({ currentPassword, newPassword }),
        },
        token
      );
      toast.success("Password changed successfully");
      setCurrentPassword("");
      setNewPassword("");
    } catch (err: any) {
      toast.error(err.message || "Failed to change password");
    } finally {
      setIsChangingPassword(false);
    }
  };

  const handleExportData = async () => {
    try {
      setIsExporting(true);
      const backendUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";
      const res = await fetch(`${backendUrl}/settings/export/transactions`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!res.ok) {
        throw new Error("Failed to export data");
      }

      const blob = await res.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `transactions_export_${new Date().toISOString().split("T")[0]}.csv`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
      toast.success("Export successful");
    } catch (err: any) {
      toast.error(err.message || "Export failed");
    } finally {
      setIsExporting(false);
    }
  };

  const handleExportAllData = async () => {
    try {
      setIsExporting(true);
      const types = Object.entries(exportTypes).filter(([_, v]) => v).map(([k, _]) => k).join(",");
      const backendUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";
      const res = await fetch(`${backendUrl}/settings/export/all?types=${types}`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!res.ok) {
        throw new Error("Failed to export backup data");
      }

      const blob = await res.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `ppbudget_backup_${new Date().toISOString().split("T")[0]}.json`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
      toast.success("Backup Export successful");
    } catch (err: any) {
      toast.error(err.message || "Export failed");
    } finally {
      setIsExporting(false);
    }
  };

  const handleDeleteAccount = async () => {
    if (!window.confirm("Are you absolutely sure? This will permanently delete all your financial data.")) {
      return;
    }

    try {
      setIsDeleting(true);
      await apiFetch(
        "/settings/account",
        {
          method: "DELETE",
        },
        token
      );
      localStorage.removeItem("ppbudget_token");
      toast.success("Account deleted successfully");
      router.push("/login");
    } catch (err: any) {
      toast.error(err.message || "Failed to delete account");
      setIsDeleting(false);
    }
  };

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


      {/* Security */}
      <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-sm overflow-hidden">
        <div className="p-6 border-b border-slate-100 dark:border-slate-800 flex items-center gap-3">
          <div className="p-2 bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 rounded-xl">
            <Lock className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-lg font-bold text-slate-900 dark:text-white">Security</h2>
            <p className="text-xs text-slate-500 dark:text-slate-400">Update your account password</p>
          </div>
        </div>
        <div className="p-6">
          <form onSubmit={handleChangePassword} className="grid grid-cols-1 md:grid-cols-2 gap-6 items-end">
            <div className="space-y-2">
              <label className="text-sm font-medium text-slate-700 dark:text-slate-300">Current Password</label>
              <Input
                type="password"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                placeholder="Enter current password"
                className="w-full bg-slate-50 dark:bg-slate-800/80 border-slate-200 dark:border-slate-700"
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium text-slate-700 dark:text-slate-300">New Password</label>
              <Input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="Enter new password"
                className="w-full bg-slate-50 dark:bg-slate-800/80 border-slate-200 dark:border-slate-700"
              />
            </div>
            <div className="md:col-span-2 flex justify-end border-t border-slate-100 dark:border-slate-800/60 pt-4 mt-2">
              <Button
                type="submit"
                disabled={isChangingPassword}
                className="bg-indigo-600 hover:bg-indigo-700 text-white min-w-[160px] rounded-xl shadow-sm"
              >
                {isChangingPassword ? "Updating..." : "Change Password"}
              </Button>
            </div>
          </form>
        </div>
      </div>

      {/* Data Management */}
      <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-sm overflow-hidden">
        <div className="p-6 border-b border-slate-100 dark:border-slate-800 flex items-center gap-3">
          <div className="p-2 bg-blue-50 dark:bg-blue-500/10 text-blue-600 dark:text-blue-400 rounded-xl">
            <Database className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-lg font-bold text-slate-900 dark:text-white">Data Management</h2>
            <p className="text-xs text-slate-500 dark:text-slate-400">Export your financial data</p>
          </div>
        </div>
        <div className="p-6">
          <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center justify-between mb-4">
            <div className="text-sm text-slate-600 dark:text-slate-400 max-w-lg leading-relaxed">
              Download a complete CSV export of all your transactions. This includes dates, amounts, categories, and account information.
            </div>
            <Button
              onClick={handleExportData}
              disabled={isExporting}
              variant="outline"
              className="flex items-center gap-2 border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800 rounded-xl shadow-sm shrink-0"
            >
              <Download className="h-4 w-4" />
              {isExporting ? "Exporting..." : "Export Transactions (CSV)"}
            </Button>
          </div>
          <div className="flex flex-col border-t border-slate-100 dark:border-slate-800 pt-4">
            <div className="text-sm text-slate-600 dark:text-slate-400 mb-4 leading-relaxed">
              Download a JSON backup of specific subsets of your data.
            </div>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 mb-6">
              {Object.entries(exportTypes).map(([key, value]) => (
                <label key={key} className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={value}
                    onChange={(e) => setExportTypes({ ...exportTypes, [key]: e.target.checked })}
                    className="rounded border-slate-300 text-indigo-600 focus:ring-indigo-600"
                  />
                  <span className="text-sm text-slate-700 dark:text-slate-300 capitalize">{key}</span>
                </label>
              ))}
            </div>
            <div className="flex justify-end">
              <Button
                onClick={handleExportAllData}
                disabled={isExporting}
                variant="outline"
                className="flex items-center gap-2 border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800 rounded-xl shadow-sm shrink-0"
              >
                <Database className="h-4 w-4" />
                {isExporting ? "Exporting..." : "Export Selected (JSON)"}
              </Button>
            </div>
          </div>
        </div>
      </div>

      {/* Email Notifications */}
      <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-sm overflow-hidden">
        <div className="p-6 border-b border-slate-100 dark:border-slate-800 flex items-center gap-3">
          <div className="p-2 bg-emerald-50 dark:bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 rounded-xl">
            <Mail className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-lg font-bold text-slate-900 dark:text-white">Email Notifications</h2>
            <p className="text-xs text-slate-500 dark:text-slate-400">Test your email delivery</p>
          </div>
        </div>
        <div className="p-6">
          <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center justify-between">
            <div className="text-sm text-slate-600 dark:text-slate-400 max-w-lg leading-relaxed">
              Send a test email notification to verify your SMTP or Resend configuration.
            </div>
            <Button
              onClick={handleTestEmail}
              disabled={isTestingEmail}
              variant="outline"
              className="flex items-center gap-2 border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800 rounded-xl shadow-sm shrink-0"
            >
              <Mail className="h-4 w-4" />
              {isTestingEmail ? "Sending..." : "Send Test Email"}
            </Button>
          </div>
        </div>
      </div>

      {/* Danger Zone */}
      <div className="bg-red-50/50 dark:bg-red-950/10 border border-red-200 dark:border-red-900/50 rounded-2xl shadow-sm overflow-hidden">
        <div className="p-6 border-b border-red-100 dark:border-red-900/30 flex items-center gap-3">
          <div className="p-2 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-xl">
            <AlertTriangle className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-lg font-bold text-red-700 dark:text-red-400">Danger Zone</h2>
            <p className="text-xs text-red-600/80 dark:text-red-400/80">Irreversible account actions</p>
          </div>
        </div>
        <div className="p-6">
          <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center justify-between">
            <div className="text-sm text-red-700/90 dark:text-red-400/90 max-w-lg font-medium">
              Permanently delete your account and all associated financial data. This action cannot be undone.
            </div>
            <Button
              onClick={handleDeleteAccount}
              disabled={isDeleting}
              variant="destructive"
              className="bg-red-600 hover:bg-red-700 text-white rounded-xl shadow-sm shrink-0"
            >
              {isDeleting ? "Deleting..." : "Delete Account"}
            </Button>
          </div>
        </div>
      </div>


    </div>
  );
}

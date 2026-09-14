"use client";

import { motion } from "framer-motion";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { NetWorthCard } from "./_components/net-worth-card";
import { CashFlowCard, SafeToSpendCard, SavingsRateCard } from "./_components/kpi-cards";
import { NeedsReviewCard, RecentTransactionsCard, TopSpendingCard, UpcomingBillsCard } from "./_components/list-cards";

export default function DashboardPage() {
  return (
    <PageContainer maxWidth="6xl">
      <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.3 }} className="space-y-6">
        <PageHeader title="Dashboard Overview" description="Quick access to your finances." className="mb-2" />

        <NetWorthCard />

        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <CashFlowCard />
          <SavingsRateCard />
          <SafeToSpendCard />
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <TopSpendingCard />
          <UpcomingBillsCard />
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <RecentTransactionsCard className="lg:col-span-2" />
          <NeedsReviewCard />
        </div>
      </motion.div>
    </PageContainer>
  );
}

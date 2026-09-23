"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

/** Retirement moved under the Wealth Strategy shell. See cash-plan/page.tsx. */
export default function RetirementRedirect() {
  const router = useRouter();
  useEffect(() => {
    router.replace("/wealth/retirement");
  }, [router]);
  return null;
}

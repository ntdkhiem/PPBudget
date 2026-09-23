"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

/**
 * Cash Plan moved under the Wealth Strategy shell.
 *
 * Kept as a redirect rather than deleted because this path has been linked from
 * the sidebar and from headline CTAs, and a bookmark landing on a 404 is a
 * worse outcome than one extra file.
 *
 * Redirects on the CLIENT, not with the server `redirect()` helper. The
 * surrounding dashboard layout is a client component that renders null until
 * its auth effect has run, so a server redirect in a streaming context -- which
 * emits its instruction as markup for the client to act on -- never reaches the
 * document. The request 302s correctly in the server log while the address bar
 * and the view both stay put, which is a confusing way to fail.
 */
export default function CashPlanRedirect() {
  const router = useRouter();
  useEffect(() => {
    router.replace("/wealth/cash");
  }, [router]);
  return null;
}

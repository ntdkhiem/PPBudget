import React from "react";

export function TransactionRow() {
  return (
    <tr className="border-b transition-colors hover:bg-muted/50 data-[state=selected]:bg-muted">
      <td className="p-4 align-middle">2023-01-01</td>
      <td className="p-4 align-middle">Example Transaction</td>
      <td className="p-4 align-middle text-right font-medium">$10.00</td>
      <td className="p-4 align-middle">
        {/* Inline review modal trigger */}
        <button className="inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 border border-input bg-background hover:bg-accent hover:text-accent-foreground h-9 px-3">
          Review
        </button>
      </td>
    </tr>
  );
}

import React from "react";

export function DataTable() {
  return (
    <div className="rounded-md border">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b bg-muted/50 hover:bg-muted/50 data-[state=selected]:bg-muted">
            <th className="h-12 px-4 text-left align-middle font-medium text-muted-foreground">Date</th>
            <th className="h-12 px-4 text-left align-middle font-medium text-muted-foreground">Description</th>
            <th className="h-12 px-4 text-right align-middle font-medium text-muted-foreground">Amount</th>
          </tr>
        </thead>
        <tbody>
          {/* Reusable transaction table rows */}
        </tbody>
      </table>
    </div>
  );
}

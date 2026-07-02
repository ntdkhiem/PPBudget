const fs = require('fs');
const path = 'frontend/app/(dashboard)/transactions/page.tsx';
let code = fs.readFileSync(path, 'utf8');

// 1. Move truncateText out of TransactionsPage
const truncateTextPattern = /  const truncateText = \(text: string, maxLength: number = 100\) => {[\s\S]*?  };\n/;
const match = code.match(truncateTextPattern);
if (match) {
  code = code.replace(match[0], '');
  // Insert it before export default function TransactionsPage
  code = code.replace(
    'export default function TransactionsPage() {',
    `${match[0].trim()}\n\nexport default function TransactionsPage() {`
  );
}

// 2. Add imports for memo at the top
if (!code.includes(', memo }')) {
  code = code.replace('import { useState, useMemo, useEffect, useRef, useCallback } from "react";', 'import { useState, useMemo, useEffect, useRef, useCallback, memo } from "react";');
}

// 3. Create the TransactionRow component
const transactionRowComponent = `
interface TransactionRowProps {
  txn: Transaction;
  isSelected: boolean;
  accounts: Account[] | undefined;
  categories: Category[] | undefined;
  quickEditTxnId: string | null;
  onQuickEditTxnIdChange: (id: string | null) => void;
  onSelectRow: (id: string, checked: boolean) => void;
  onRowClick: (txn: Transaction) => void;
  onDelete: (id: string, e: React.MouseEvent) => void;
  onReview: (id: string, categoryId: string) => void;
  isDeleting: boolean;
}

const TransactionRow = memo(function TransactionRow({
  txn,
  isSelected,
  accounts,
  categories,
  quickEditTxnId,
  onQuickEditTxnIdChange,
  onSelectRow,
  onRowClick,
  onDelete,
  onReview,
  isDeleting,
}: TransactionRowProps) {
  return (
    <TableRow 
      className={\`group cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 \${isSelected ? 'bg-indigo-50 dark:bg-indigo-900/30' : ''}\`}
      onClick={() => onRowClick(txn)}
    >
      <TableCell className="w-12 py-4 text-center" onClick={(e) => e.stopPropagation()}>
        <input 
          type="checkbox" 
          className="w-4 h-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-600 cursor-pointer"
          checked={isSelected}
          onChange={(e) => onSelectRow(txn.id, e.target.checked)}
        />
      </TableCell>
      <TableCell className="py-4 font-medium text-slate-900 dark:text-slate-100 max-w-xs" title={txn.description}>
        <div className="line-clamp-3 whitespace-pre-wrap break-words">{truncateText(txn.description)}</div>
        <div className="mt-2 flex flex-wrap gap-1.5 items-center">
          {!txn.is_reviewed && (
            <span className="inline-flex items-center rounded-full bg-amber-100/80 dark:bg-amber-500/20 border border-amber-200 dark:border-amber-500/30 px-2.5 py-0.5 text-xs font-semibold text-amber-700 dark:text-amber-400">
              Needs Review
            </span>
          )}
          {txn.subscription_id && (
            <span className="inline-flex items-center gap-1 rounded-full bg-indigo-100/80 dark:bg-indigo-500/20 border border-indigo-200 dark:border-indigo-500/30 px-2.5 py-0.5 text-xs font-semibold text-indigo-700 dark:text-indigo-400" title="Subscription Payment">
              <Repeat size={12} /> Subscription
            </span>
          )}
        </div>
      </TableCell>
      <TableCell className="text-right py-4 font-semibold">
        <div className="flex flex-col items-end gap-0.5">
          <span className={txn.amount < 0 ? "text-rose-600 dark:text-rose-400" : "text-emerald-600 dark:text-emerald-400"}>
            {formatCurrency(txn.amount)}
          </span>
          {txn.pays_for && txn.pays_for.length > 0 && (
            <span className="text-xs font-medium text-indigo-600 dark:text-indigo-400 bg-indigo-50 dark:bg-indigo-900/30 px-1.5 py-0.5 rounded flex items-center gap-1" title={\`Pays for \${txn.pays_for.length} transaction(s).\`}>
              <Link size={12} /> Pays for {txn.pays_for.length} txn{txn.pays_for.length !== 1 ? 's' : ''} (Effective: {formatCurrency(txn.effective_amount!)})
            </span>
          )}
          {txn.paid_by && txn.paid_by.length > 0 && (
            <span className="text-xs font-medium text-slate-500 bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 rounded flex items-center gap-1" title={\`Paid by \${txn.paid_by.length} transaction(s).\`}>
              <Link size={12} /> Paid by {txn.paid_by.length} txn{txn.paid_by.length !== 1 ? 's' : ''} (Effective: {formatCurrency(txn.effective_amount!)})
            </span>
          )}
        </div>
      </TableCell>
      <TableCell className="py-4 text-slate-600 dark:text-slate-300 whitespace-nowrap">
        {formatDate(txn.date)}
      </TableCell>
      <TableCell className="py-4 text-slate-600 dark:text-slate-300 max-w-[150px]" title={accounts?.find(a => a.id === txn.account_id)?.name || "Unknown"}>
        <div className="line-clamp-3 whitespace-pre-wrap break-words">{truncateText(accounts?.find(a => a.id === txn.account_id)?.name || "Unknown")}</div>
      </TableCell>
      <TableCell className="py-4" onClick={(e) => e.stopPropagation()}>
        <Popover open={quickEditTxnId === txn.id} onOpenChange={(open) => onQuickEditTxnIdChange(open ? txn.id : null)}>
          <PopoverTrigger asChild>
            <Button variant="ghost" className={\`h-8 px-3 rounded-lg text-sm font-medium \${txn.category_id ? 'text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700' : 'text-indigo-600 bg-indigo-50 dark:bg-indigo-900/30 hover:bg-indigo-100 dark:text-indigo-400 dark:bg-indigo-500/10 dark:hover:bg-indigo-500/20'}\`}>
              {categories?.find((c) => c.id === txn.category_id)?.name || "Uncategorized"}
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-56 p-2 rounded-xl" align="start">
            <div className="space-y-1">
              <h4 className="font-medium text-sm px-2 py-1.5 text-slate-500">Quick Edit Category</h4>
              <div className="max-h-60 overflow-y-auto">
                {categories?.map((cat) => (
                  <div
                    key={cat.id}
                    className={\`px-2 py-1.5 text-sm rounded-md cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 dark:bg-slate-800 dark:hover:bg-slate-800 flex items-center justify-between \${txn.category_id === cat.id ? 'bg-indigo-50 dark:bg-indigo-500/10 text-indigo-600 dark:text-indigo-400' : ''}\`}
                    onClick={() => onReview(txn.id, cat.id)}
                  >
                    {cat.name}
                    {txn.category_id === cat.id && <CheckCircle2 className="h-4 w-4" />}
                  </div>
                ))}
              </div>
            </div>
          </PopoverContent>
        </Popover>
      </TableCell>
      <TableCell className="text-right py-4" onClick={(e) => e.stopPropagation()}>
        <div className="flex justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          <Button
            variant="ghost"
            size="icon"
            aria-label="Edit transaction"
            className="text-slate-400 hover:text-indigo-600 hover:bg-indigo-50 dark:bg-indigo-900/30 dark:hover:text-indigo-400 dark:hover:bg-indigo-500/10 h-8 w-8 rounded-lg"
            onClick={(e) => { e.stopPropagation(); onRowClick(txn); }}
          >
            <Edit2 className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Delete transaction"
            className="text-slate-400 hover:text-rose-600 hover:bg-rose-50 dark:hover:text-rose-400 dark:hover:bg-rose-500/10 h-8 w-8 rounded-lg"
            onClick={(e) => onDelete(txn.id, e)}
            disabled={isDeleting}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </TableCell>
    </TableRow>
  );
});
`;

code = code.replace('export default function TransactionsPage() {', transactionRowComponent + '\nexport default function TransactionsPage() {');

// 4. Update the map loop
const mapStart = `                  <TableRow 
                    key={virtualRow.key} 
                    className={\`group cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 \${selectedIds.includes(txn.id) ? 'bg-indigo-50 dark:bg-indigo-900/30' : ''}\`}
                    onClick={() => handleRowClick(txn)}
                  >`;

// The ending is right before the closing tag of the map loop.
// We should use regex to replace the entire TableRow block inside the map.
// Actually, it's easier to find the exact block since it's quite large. Let's just string match.

// Wait, the block starts at `                  <TableRow` and ends at `    </TableRow>`.
const fullMapTarget = `                  <TableRow 
                    key={virtualRow.key} 
                    className={\`group cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 \${selectedIds.includes(txn.id) ? 'bg-indigo-50 dark:bg-indigo-900/30' : ''}\`}
                    onClick={() => handleRowClick(txn)}
                  >
                  <TableCell className="w-12 py-4 text-center" onClick={(e) => e.stopPropagation()}>
                    <input 
                      type="checkbox" 
                      className="w-4 h-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-600 cursor-pointer"
                      checked={selectedIds.includes(txn.id)}
                      onChange={(e) => handleSelectRow(txn.id, e.target.checked)}
                    />
                  </TableCell>
                  <TableCell className="py-4 font-medium text-slate-900 dark:text-slate-100 max-w-xs" title={txn.description}>
                    <div className="line-clamp-3 whitespace-pre-wrap break-words">{truncateText(txn.description)}</div>
                    <div className="mt-2 flex flex-wrap gap-1.5 items-center">
                      {!txn.is_reviewed && (
                        <span className="inline-flex items-center rounded-full bg-amber-100/80 dark:bg-amber-500/20 border border-amber-200 dark:border-amber-500/30 px-2.5 py-0.5 text-xs font-semibold text-amber-700 dark:text-amber-400">
                          Needs Review
                        </span>
                      )}
                      {txn.subscription_id && (
                        <span className="inline-flex items-center gap-1 rounded-full bg-indigo-100/80 dark:bg-indigo-500/20 border border-indigo-200 dark:border-indigo-500/30 px-2.5 py-0.5 text-xs font-semibold text-indigo-700 dark:text-indigo-400" title="Subscription Payment">
                          <Repeat size={12} /> Subscription
                        </span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="text-right py-4 font-semibold">
                    <div className="flex flex-col items-end gap-0.5">
                      <span className={txn.amount < 0 ? "text-rose-600 dark:text-rose-400" : "text-emerald-600 dark:text-emerald-400"}>
                        {formatCurrency(txn.amount)}
                      </span>
                      {txn.pays_for && txn.pays_for.length > 0 && (
                        <span className="text-xs font-medium text-indigo-600 dark:text-indigo-400 bg-indigo-50 dark:bg-indigo-900/30 px-1.5 py-0.5 rounded flex items-center gap-1" title={\`Pays for \${txn.pays_for.length} transaction(s).\`}>
                          <Link size={12} /> Pays for {txn.pays_for.length} txn{txn.pays_for.length !== 1 ? 's' : ''} (Effective: {formatCurrency(txn.effective_amount!)})
                        </span>
                      )}
                      {txn.paid_by && txn.paid_by.length > 0 && (
                        <span className="text-xs font-medium text-slate-500 bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 rounded flex items-center gap-1" title={\`Paid by \${txn.paid_by.length} transaction(s).\`}>
                          <Link size={12} /> Paid by {txn.paid_by.length} txn{txn.paid_by.length !== 1 ? 's' : ''} (Effective: {formatCurrency(txn.effective_amount!)})
                        </span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="py-4 text-slate-600 dark:text-slate-300 whitespace-nowrap">
                    {formatDate(txn.date)}
                  </TableCell>
                  <TableCell className="py-4 text-slate-600 dark:text-slate-300 max-w-[150px]" title={accounts?.find(a => a.id === txn.account_id)?.name || "Unknown"}>
                    <div className="line-clamp-3 whitespace-pre-wrap break-words">{truncateText(accounts?.find(a => a.id === txn.account_id)?.name || "Unknown")}</div>
                  </TableCell>
                  <TableCell className="py-4" onClick={(e) => e.stopPropagation()}>
                    <Popover open={quickEditTxnId === txn.id} onOpenChange={(open) => setQuickEditTxnId(open ? txn.id : null)}>
                      <PopoverTrigger asChild>
                        <Button variant="ghost" className={\`h-8 px-3 rounded-lg text-sm font-medium \${txn.category_id ? 'text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700' : 'text-indigo-600 bg-indigo-50 dark:bg-indigo-900/30 hover:bg-indigo-100 dark:text-indigo-400 dark:bg-indigo-500/10 dark:hover:bg-indigo-500/20'}\`}>
                          {categories?.find((c) => c.id === txn.category_id)?.name || "Uncategorized"}
                        </Button>
                      </PopoverTrigger>
                      <PopoverContent className="w-56 p-2 rounded-xl" align="start">
                        <div className="space-y-1">
                          <h4 className="font-medium text-sm px-2 py-1.5 text-slate-500">Quick Edit Category</h4>
                          <div className="max-h-60 overflow-y-auto">
                            {categories?.map((cat) => (
                              <div
                                key={cat.id}
                                className={\`px-2 py-1.5 text-sm rounded-md cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 dark:bg-slate-800 dark:hover:bg-slate-800 flex items-center justify-between \${txn.category_id === cat.id ? 'bg-indigo-50 dark:bg-indigo-500/10 text-indigo-600 dark:text-indigo-400' : ''}\`}
                                onClick={() => reviewMutation.mutate({ id: txn.id, categoryId: cat.id })}
                              >
                                {cat.name}
                                {txn.category_id === cat.id && <CheckCircle2 className="h-4 w-4" />}
                              </div>
                            ))}
                          </div>
                        </div>
                      </PopoverContent>
                    </Popover>
                  </TableCell>
                  <TableCell className="text-right py-4" onClick={(e) => e.stopPropagation()}>
                    <div className="flex justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Edit transaction"
                        className="text-slate-400 hover:text-indigo-600 hover:bg-indigo-50 dark:bg-indigo-900/30 dark:hover:text-indigo-400 dark:hover:bg-indigo-500/10 h-8 w-8 rounded-lg"
                        onClick={(e) => { e.stopPropagation(); handleRowClick(txn); }}
                      >
                        <Edit2 className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Delete transaction"
                        className="text-slate-400 hover:text-rose-600 hover:bg-rose-50 dark:hover:text-rose-400 dark:hover:bg-rose-500/10 h-8 w-8 rounded-lg"
                        onClick={(e) => handleDelete(txn.id, e)}
                        disabled={deleteMutation.isPending}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>`;

const fullMapReplacement = `                  <TransactionRow 
                    key={virtualRow.key}
                    txn={txn}
                    isSelected={selectedIds.includes(txn.id)}
                    accounts={accounts}
                    categories={categories}
                    quickEditTxnId={quickEditTxnId}
                    onQuickEditTxnIdChange={setQuickEditTxnId}
                    onSelectRow={handleSelectRow}
                    onRowClick={handleRowClick}
                    onDelete={handleDelete}
                    onReview={handleReview}
                    isDeleting={deleteMutation.isPending}
                  />`;

code = code.replace(fullMapTarget, fullMapReplacement);

// We need to define handleReview in TransactionsPage!
const handleReviewTarget = `  const handleSelectAll = (checked: boolean) => {`;
const handleReviewReplacement = `  const handleReview = useCallback((id: string, categoryId: string) => {
    reviewMutation.mutate({ id, categoryId });
  }, [reviewMutation]);

  const handleSelectAll = (checked: boolean) => {`;
  
code = code.replace(handleReviewTarget, handleReviewReplacement);

fs.writeFileSync(path, code);
console.log("Extracted MemoizedTransactionRow!");

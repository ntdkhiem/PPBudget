const fs = require('fs');
const path = 'frontend/app/(dashboard)/transactions/page.tsx';
let code = fs.readFileSync(path, 'utf8');

// 1. Add imports
code = code.replace(
  `import { useState, useMemo, useEffect } from "react";`,
  `import { useState, useMemo, useEffect, useRef, useCallback } from "react";\nimport { useVirtualizer } from "@tanstack/react-virtual";`
);

// 2. Add virtualizer setup to TransactionsPage
const target1 = `export default function TransactionsPage() {\n  const router =`;
code = code.replace(
  target1,
  `export default function TransactionsPage() {\n  const parentRef = useRef<HTMLDivElement>(null);\n  const router =`
);

const targetReturn = `  return (\n    <motion.div`;
const virtualizerSetup = `
  const rowVirtualizer = useVirtualizer({
    count: filteredTransactions.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 64, // Approximate row height
    overscan: 10,
  });

`;
code = code.replace(targetReturn, virtualizerSetup + targetReturn);

// 3. Virtualize the main Table
const tableStart = `<Table className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800/60 rounded-xl overflow-hidden shadow-sm">`;
code = code.replace(tableStart, `<div ref={parentRef} className="h-[600px] overflow-auto relative rounded-xl border border-slate-200 dark:border-slate-800/60 shadow-sm">\n            <Table className="bg-white dark:bg-slate-900 w-full relative">`);

const tableBodyStart = `<TableBody>`;
// Only replace the FIRST TableBody (which is the main table)
code = code.replace(tableBodyStart, `<TableBody style={{ height: \`\${rowVirtualizer.getTotalSize()}px\`, position: 'relative' }}>`);

const mapTarget = `filteredTransactions.map((txn) => (
                <TableRow 
                  key={txn.id} 
                  className={\`group cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 \${selectedIds.includes(txn.id) ? 'bg-indigo-50 dark:bg-indigo-900/30' : ''}\`}
                  onClick={() => handleRowClick(txn)}
                >`;

const mapReplacement = `rowVirtualizer.getVirtualItems().map((virtualRow) => {
                const txn = filteredTransactions[virtualRow.index];
                return (
                <TableRow 
                  key={virtualRow.key} 
                  className={\`group cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 absolute top-0 left-0 w-full flex items-center \${selectedIds.includes(txn.id) ? 'bg-indigo-50 dark:bg-indigo-900/30' : ''}\`}
                  style={{
                    height: \`\${virtualRow.size}px\`,
                    transform: \`translateY(\${virtualRow.start}px)\`,
                  }}
                  onClick={() => handleRowClick(txn)}
                >`;

code = code.replace(mapTarget, mapReplacement);

// Fix the closing parenthesis for the map
const mapEndTarget = `                  </TableCell>
                </TableRow>
              ))
            )}`;
const mapEndReplacement = `                  </TableCell>
                </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </div>`;

// Actually we need to replace the closing </Table> for the main table only.
// Let's replace the whole block up to </Table>
code = code.replace(mapEndTarget, `                  </TableCell>
                </TableRow>
                );
              })
            )}`);

// Then find the NEXT </Table> after the map and replace with </Table></div>
// A simpler way: we just replaced `)) \n )}`. 
// So let's look for:
const tableCloseTarget = `            )}
          </TableBody>
        </Table>`;

// We will only replace the FIRST occurrence of this exact pattern
code = code.replace(tableCloseTarget, `            )}
          </TableBody>
        </Table></div>`);

fs.writeFileSync(path, code);
console.log("Patched successfully!");

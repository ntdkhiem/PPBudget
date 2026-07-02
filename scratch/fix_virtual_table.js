const fs = require('fs');
const path = 'frontend/app/(dashboard)/transactions/page.tsx';
let code = fs.readFileSync(path, 'utf8');

// 1. Remove style from TableBody
code = code.replace(
  `<TableBody style={{ height: \`\${rowVirtualizer.getTotalSize()}px\`, position: 'relative' }}>`,
  `<TableBody>`
);

// 2. Replace the mapping
const mapTarget = `rowVirtualizer.getVirtualItems().map((virtualRow) => {
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

const mapReplacement = `
              <>
                {rowVirtualizer.getVirtualItems().length > 0 && (
                  <TableRow style={{ height: \`\${rowVirtualizer.getVirtualItems()[0]?.start || 0}px\` }} className="hover:bg-transparent pointer-events-none border-none">
                    <TableCell colSpan={7} className="p-0 border-none" />
                  </TableRow>
                )}
                {rowVirtualizer.getVirtualItems().map((virtualRow) => {
                  const txn = filteredTransactions[virtualRow.index];
                  return (
                  <TableRow 
                    key={virtualRow.key} 
                    className={\`group cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 \${selectedIds.includes(txn.id) ? 'bg-indigo-50 dark:bg-indigo-900/30' : ''}\`}
                    onClick={() => handleRowClick(txn)}
                  >`;

code = code.replace(mapTarget, mapReplacement);

// 3. We also need to add the bottom spacer row after the map.
// The map closes with:
const mapEndTarget = `                  </TableCell>
                </TableRow>
                );
              })
            )}`;

const mapEndReplacement = `                  </TableCell>
                </TableRow>
                );
              })}
              {rowVirtualizer.getVirtualItems().length > 0 && (
                <TableRow style={{ height: \`\${rowVirtualizer.getTotalSize() - (rowVirtualizer.getVirtualItems()[rowVirtualizer.getVirtualItems().length - 1]?.end || 0)}px\` }} className="hover:bg-transparent pointer-events-none border-none">
                  <TableCell colSpan={7} className="p-0 border-none" />
                </TableRow>
              )}
            </>
            )}`;

code = code.replace(mapEndTarget, mapEndReplacement);

fs.writeFileSync(path, code);
console.log("Fixed table layout!");

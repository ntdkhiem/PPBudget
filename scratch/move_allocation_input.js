const fs = require('fs');

const pagePath = 'frontend/app/(dashboard)/transactions/page.tsx';
let pageCode = fs.readFileSync(pagePath, 'utf8');

const editDialogPath = 'frontend/app/(dashboard)/transactions/EditTransactionDialog.tsx';
let editDialogCode = fs.readFileSync(editDialogPath, 'utf8');

// 1. Extract AllocationAmountInput from page.tsx
const inputTarget = /function AllocationAmountInput\(\{[\s\S]*?\}\) \{[\s\S]*?return \([\s\S]*?<\/div>\n  \);\n\}\n/;
const inputMatch = pageCode.match(inputTarget);
let inputCode = '';
if (inputMatch) {
  inputCode = inputMatch[0];
  pageCode = pageCode.replace(inputTarget, '');
}

fs.writeFileSync(pagePath, pageCode);

// 2. Insert AllocationAmountInput into EditTransactionDialog.tsx
// Put it right before function TransactionAllocationList
if (inputCode) {
  editDialogCode = editDialogCode.replace(
    'function TransactionAllocationList',
    inputCode + '\n\nfunction TransactionAllocationList'
  );
  
  // also add useEffect to imports in EditTransactionDialog.tsx if not present
  if (!editDialogCode.includes('useEffect')) {
    editDialogCode = editDialogCode.replace('import { useState }', 'import { useState, useEffect }');
  }

  fs.writeFileSync(editDialogPath, editDialogCode);
}
console.log("Moved AllocationAmountInput!");

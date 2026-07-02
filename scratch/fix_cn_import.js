const fs = require('fs');
const path = 'frontend/app/(dashboard)/transactions/EditTransactionDialog.tsx';
let code = fs.readFileSync(path, 'utf8');

// Add cn to the import from "@/lib/utils"
// Currently it is: import { formatCurrency, formatDate } from "@/lib/utils";
code = code.replace(
  'import { formatCurrency, formatDate } from "@/lib/utils";',
  'import { formatCurrency, formatDate, cn } from "@/lib/utils";'
);

fs.writeFileSync(path, code);
console.log("Added cn import to EditTransactionDialog.tsx");

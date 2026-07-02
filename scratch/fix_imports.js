const fs = require('fs');
const path = 'frontend/app/(dashboard)/transactions/page.tsx';
let code = fs.readFileSync(path, 'utf8');

// Move import TransactionFilters to the top
if (code.includes('import TransactionFilters from "./TransactionFilters";')) {
  code = code.replace('import TransactionFilters from "./TransactionFilters";', '');
  code = code.replace(
    'import dynamic from "next/dynamic";',
    'import dynamic from "next/dynamic";\nimport TransactionFilters from "./TransactionFilters";'
  );
  fs.writeFileSync(path, code);
  console.log("Moved import TransactionFilters to top");
}

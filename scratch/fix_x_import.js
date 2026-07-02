const fs = require('fs');
const path = 'frontend/app/(dashboard)/transactions/EditTransactionDialog.tsx';
let code = fs.readFileSync(path, 'utf8');

code = code.replace(
  'import { Loader2, Link, Check, ChevronsUpDown } from "lucide-react";',
  'import { Loader2, Link, Check, ChevronsUpDown, X } from "lucide-react";'
);

fs.writeFileSync(path, code);
console.log("Added X import to EditTransactionDialog.tsx");

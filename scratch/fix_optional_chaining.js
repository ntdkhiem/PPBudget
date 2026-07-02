const fs = require('fs');

const path = 'frontend/app/(dashboard)/transactions/EditTransactionDialog.tsx';
let code = fs.readFileSync(path, 'utf8');

// Fix optional chaining
code = code.replace(
  /transactions\?\.slice\(0, 50\)\.map/g,
  'transactions?.slice(0, 50)?.map'
);

fs.writeFileSync(path, code);
console.log("Fixed optional chaining in EditTransactionDialog.tsx");

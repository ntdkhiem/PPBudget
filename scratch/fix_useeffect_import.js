const fs = require('fs');
const path = 'frontend/app/(dashboard)/transactions/EditTransactionDialog.tsx';
let code = fs.readFileSync(path, 'utf8');

code = code.replace(
  'import { useState } from "react";',
  'import { useState, useEffect } from "react";'
);

fs.writeFileSync(path, code);
console.log("Added useEffect import to EditTransactionDialog.tsx");

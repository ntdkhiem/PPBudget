const fs = require('fs');

const path = 'frontend/app/(dashboard)/transactions/page.tsx';
let code = fs.readFileSync(path, 'utf8');

// Add the missing useEffect for selectedTxn
const target = `  useEffect(() => {
    const editId = searchParams.get("edit");`;

const replacement = `  useEffect(() => {
    if (selectedTxn) {
      setPaidBy(selectedTxn.paid_by || []);
      setPaysFor(selectedTxn.pays_for || []);
    }
  }, [selectedTxn]);

  useEffect(() => {
    const editId = searchParams.get("edit");`;

if (!code.includes('setPaidBy(selectedTxn.paid_by || []);')) {
  code = code.replace(target, replacement);
  fs.writeFileSync(path, code);
  console.log("Restored missing useEffect for selectedTxn");
} else {
  console.log("useEffect already exists");
}

const fs = require('fs');
const path = 'frontend/app/(dashboard)/transactions/page.tsx';
let code = fs.readFileSync(path, 'utf8');

// The block to replace:
// It starts at `<Dialog open={isAddOpen}` and ends at the corresponding `</Dialog>`
const addDialogTarget = /<Dialog open={isAddOpen} onOpenChange={setIsAddOpen}>[\s\S]*?<\/form>\s*<\/DialogContent>\s*<\/Dialog>/;

const addDialogReplacement = `<AddTransactionDialog
          isOpen={isAddOpen}
          onOpenChange={setIsAddOpen}
          onSubmit={handleCreateSubmit}
          isPending={createMutation.isPending}
          accounts={accounts}
          categories={categories}
          subscriptions={subscriptions}
        />`;

code = code.replace(addDialogTarget, addDialogReplacement);

// We need to import AddTransactionDialog at the top.
// Add it after the other imports.
const importTarget = `import { useDateRange } from "@/app/contexts/DateRangeContext";`;
const importReplacement = `import { useDateRange } from "@/app/contexts/DateRangeContext";\nimport dynamic from "next/dynamic";\n\nconst AddTransactionDialog = dynamic(() => import('./AddTransactionDialog'), { ssr: false });`;

if (!code.includes('import AddTransactionDialog')) {
    code = code.replace(importTarget, importReplacement);
}

fs.writeFileSync(path, code);
console.log("Replaced Add Dialog!");

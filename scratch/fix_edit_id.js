const fs = require('fs');
const path = 'frontend/components/global-search.tsx';
let code = fs.readFileSync(path, 'utf8');

if (code.includes('?edit_id=')) {
  code = code.replace(/\?edit_id=/g, '?edit=');
  fs.writeFileSync(path, code);
  console.log("Fixed edit_id to edit in global-search.tsx");
}

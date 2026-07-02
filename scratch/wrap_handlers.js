const fs = require('fs');
const path = 'frontend/app/(dashboard)/transactions/page.tsx';
let code = fs.readFileSync(path, 'utf8');

const selectRowTarget = `  const handleSelectRow = (id: string, checked: boolean) => {
    if (checked) {
      setSelectedIds(prev => [...prev, id]);
    } else {
      setSelectedIds(prev => prev.filter(x => x !== id));
    }
  };`;

const selectRowReplacement = `  const handleSelectRow = useCallback((id: string, checked: boolean) => {
    if (checked) {
      setSelectedIds(prev => [...prev, id]);
    } else {
      setSelectedIds(prev => prev.filter(x => x !== id));
    }
  }, []);`;

code = code.replace(selectRowTarget, selectRowReplacement);

const deleteTarget = `  const handleDelete = (id: string, e?: React.MouseEvent) => {
    e?.stopPropagation();
    if (confirm("Are you sure you want to delete this transaction?")) {
      deleteMutation.mutate(id);
    }
  };`;

const deleteReplacement = `  const handleDelete = useCallback((id: string, e?: React.MouseEvent) => {
    e?.stopPropagation();
    if (confirm("Are you sure you want to delete this transaction?")) {
      deleteMutation.mutate(id);
    }
  }, [deleteMutation]);`;

code = code.replace(deleteTarget, deleteReplacement);

const rowClickTarget = `  const handleRowClick = (txn: Transaction) => {
    setSelectedTxn(txn);
    setIsEditOpen(true);
  };`;

const rowClickReplacement = `  const handleRowClick = useCallback((txn: Transaction) => {
    setSelectedTxn(txn);
    setIsEditOpen(true);
  }, []);`;

code = code.replace(rowClickTarget, rowClickReplacement);

fs.writeFileSync(path, code);
console.log("Wrapped handlers in useCallback!");

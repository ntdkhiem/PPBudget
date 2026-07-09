async function test() {
  const loginRes = await fetch('http://localhost:8080/api/v1/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password: 'adminpassword' })
  });
  const { token } = await loginRes.json();
  
  const txnRes = await fetch('http://localhost:8080/api/v1/transactions?start_date=2026-06-01&end_date=2026-07-31', {
    headers: { 'Authorization': `Bearer ${token}` }
  });
  const txns = await txnRes.json();
  const plusReloc = txns.find(t => t.description.includes('PLUS RELOCATION'));
  console.log("ListTransactions response for PLUS RELOC:");
  console.log(JSON.stringify(plusReloc, null, 2));

  const singleRes = await fetch(`http://localhost:8080/api/v1/transactions/${plusReloc.id}`, {
    headers: { 'Authorization': `Bearer ${token}` }
  });
  const single = await singleRes.json();
  console.log("\nGetTransaction response:");
  console.log(JSON.stringify(single, null, 2));
}
test();

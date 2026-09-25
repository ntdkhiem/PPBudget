-- 0030_account_roles.up.sql
--
-- What each account IS, as distinct from which side of the balance sheet it
-- sits on.
--
-- `type` says asset or liability, which is all net worth needs and not enough
-- for a plan: a 401(k) and a checking account are both assets, so the
-- emergency fund counted retirement money as cash; and a card imported through
-- SimpleFin arrived as an asset, invisible to every debt rule. Two stopgaps
-- grew around the gap -- a hand-picked list of cash accounts on the wealth
-- profile, and treating accounts linked on the Retirement tab as invested --
-- and this column replaces both.
--
-- NULL means "not classified yet". The engine counts an unclassified account
-- as neither cash nor investment and asks about it, rather than guessing.

BEGIN;

ALTER TABLE accounts ADD COLUMN role TEXT CHECK (role IN (
    'checking', 'savings', 'investment', 'property',
    'credit_card', 'loan', 'mortgage'
));

-- The role decides the side of the balance sheet, so the two cannot disagree.
ALTER TABLE accounts ADD CONSTRAINT accounts_role_matches_type CHECK (
    role IS NULL
    OR (role IN ('credit_card', 'loan', 'mortgage') AND type = 'liability')
    OR (role IN ('checking', 'savings', 'investment', 'property') AND type = 'asset')
);

-- Backfill. The keyword rules mirror inferAccountKind in
-- internal/service/account_roles.go, which classifies accounts imported from
-- now on.

-- 1. Accounts already linked to a tax treatment are investments.
UPDATE accounts a SET role = 'investment'
FROM retirement_account_terms r
WHERE r.account_id = a.id AND a.type = 'asset';

-- 2. Liabilities, by name. An unrecognised one is most likely a card.
UPDATE accounts SET role = CASE
        WHEN name ~* '(mortgage|home\s*loan|heloc)' THEN 'mortgage'
        WHEN name ~* '(loan|lending|financ|\mauto\M|\mcar\M|student)' THEN 'loan'
        ELSE 'credit_card'
    END
WHERE type = 'liability' AND role IS NULL;

-- 3. Assets, by name. Investment keywords come first: a "Health Savings
--    Account" is an HSA, not a savings account.
UPDATE accounts SET role = CASE
        WHEN name ~* '(401|403\s*\(?b|457|\mira\M|roth|\mhsa\M|health\s*sav|brokerage|invest|retire|pension|\mtsp\M|529)' THEN 'investment'
        WHEN name ~* '(checking|\mchk\M)' THEN 'checking'
        WHEN name ~* '(saving|hysa|high[\s-]*yield|money\s*market|\mcd\M)' THEN 'savings'
        WHEN name ~* '(\mhouse\M|\mhome\M|property|real\s*estate|vehicle|\mcar\M)' THEN 'property'
    END
WHERE type = 'asset' AND role IS NULL;

-- 4. A user who picked their cash accounts by hand keeps that choice: picked
--    accounts are cash, and unpicked ones stop being cash.
UPDATE accounts a SET role = 'savings'
FROM wealth_profiles p
WHERE p.user_id = a.user_id
  AND a.type = 'asset'
  AND a.id = ANY (p.override_liquid_account_ids)
  AND (a.role IS NULL OR a.role NOT IN ('checking', 'savings'));

UPDATE accounts a SET role = NULL
FROM wealth_profiles p
WHERE p.user_id = a.user_id
  AND a.type = 'asset'
  AND cardinality(p.override_liquid_account_ids) > 0
  AND NOT (a.id = ANY (p.override_liquid_account_ids))
  AND a.role IN ('checking', 'savings');

-- 5. Every other asset still unclassified was counted as cash before this and
--    stays that way, unless its owner had picked a list.
UPDATE accounts a SET role = 'savings'
WHERE a.type = 'asset' AND a.role IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM wealth_profiles p
      WHERE p.user_id = a.user_id AND cardinality(p.override_liquid_account_ids) > 0
  );

-- The hand-picked list now lives in the roles themselves.
DELETE FROM wealth_profile_fields WHERE field_key = 'override_liquid_account_ids';
ALTER TABLE wealth_profiles DROP COLUMN override_liquid_account_ids;

COMMIT;

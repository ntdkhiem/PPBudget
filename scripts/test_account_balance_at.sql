-- test_account_balance_at.sql
--
-- Plan section 4.1 cases for account_balance_at(). Requires migration 0019+.
-- Creates its own fixtures inside a transaction and ROLLS BACK at the end, so
-- it leaves no data behind. Any failing ASSERT raises and aborts the script.
--
-- Usage: psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/test_account_balance_at.sql

BEGIN;

-- Fixtures: user U, account X (opening 1000), account Y (no opening).
INSERT INTO users (id, email, password_hash)
VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'balance-test@example.invalid', '');

INSERT INTO accounts (id, user_id, name, type)
VALUES ('aaaaaaaa-0000-0000-0000-0000000000a1', 'aaaaaaaa-0000-0000-0000-000000000001', 'Test X', 'asset'),
       ('aaaaaaaa-0000-0000-0000-0000000000b1', 'aaaaaaaa-0000-0000-0000-000000000001', 'Test Y', 'asset');

INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000a1', '-infinity', 1000, 'opening');

INSERT INTO transactions (user_id, account_id, amount, date, description)
VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000a1',  100, '2026-01-10', 'x +100'),
       ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000a1',  -50, '2026-01-20', 'x -50'),
       ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000a1',   30, '2026-02-05', 'x +30'),
       ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000b1',  100, '2026-02-20', 'y +100'),
       ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000b1',  -40, '2026-02-25', 'y -40');

-- Case 1: opening only.
DO $$
DECLARE x UUID := 'aaaaaaaa-0000-0000-0000-0000000000a1';
BEGIN
    ASSERT account_balance_at(x, '2026-01-15') = 1100, 'case1 01-15';
    ASSERT account_balance_at(x, 'infinity') = 1080, 'case1 infinity';
    ASSERT account_balance_at(x, '2026-01-10') = 1100, 'case1 end-of-day inclusive';
    ASSERT account_balance_at(x, '2026-01-09') = 1000, 'case1 before first txn';
    ASSERT account_balance_at(x, '-infinity') = 1000, 'case1 -infinity';
    RAISE NOTICE 'case 1 passed';
END $$;

-- Case 2: simplefin snapshot 5000 @ 01-31.
INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000a1', '2026-01-31', 5000, 'simplefin');

DO $$
DECLARE x UUID := 'aaaaaaaa-0000-0000-0000-0000000000a1';
BEGIN
    ASSERT account_balance_at(x, '2026-01-31') = 5000, 'case2 01-31';
    ASSERT account_balance_at(x, '2026-02-10') = 5030, 'case2 02-10';
    ASSERT account_balance_at(x, '2026-01-15') = 1100, 'case2 01-15';
    ASSERT account_balance_at(x, 'infinity') = 5030, 'case2 infinity';
    RAISE NOTICE 'case 2 passed';
END $$;

-- Case 3: account Y, no opening, snapshot 2000 @ 03-01.
INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000b1', '2026-03-01', 2000, 'simplefin');

DO $$
DECLARE y UUID := 'aaaaaaaa-0000-0000-0000-0000000000b1';
BEGIN
    ASSERT account_balance_at(y, '2026-02-22') = 2040, 'case3 02-22';
    ASSERT account_balance_at(y, '2026-02-10') = 1940, 'case3 02-10';
    ASSERT account_balance_at(y, '2026-03-05') = 2000, 'case3 03-05';
    ASSERT account_balance_at(y, '2026-03-01') = 2000, 'case3 03-01';
    ASSERT account_balance_at(y, 'infinity') = 2000, 'case3 infinity';
    ASSERT account_balance_at(y, '-infinity') = 1940, 'case3 -infinity';
    RAISE NOTICE 'case 3 passed';
END $$;

-- Case 4: soft-deleted txns ignored in all branches.
INSERT INTO transactions (user_id, account_id, amount, date, description, deleted_at)
VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000a1', 9999, '2026-01-12', 'x deleted (branch 1, before anchor)', NOW()),
       ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000a1', 7777, '2026-02-07', 'x deleted (branch 1, after anchor)', NOW()),
       ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000b1', 8888, '2026-02-21', 'y deleted (branch 2)', NOW());

-- Branch 3 fixture: account Z with no snapshots at all.
INSERT INTO accounts (id, user_id, name, type)
VALUES ('aaaaaaaa-0000-0000-0000-0000000000c1', 'aaaaaaaa-0000-0000-0000-000000000001', 'Test Z', 'asset');
INSERT INTO transactions (user_id, account_id, amount, date, description, deleted_at)
VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000c1',  250, '2026-01-05', 'z +250', NULL),
       ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000c1', 5555, '2026-01-06', 'z deleted', NOW());

DO $$
DECLARE
    x UUID := 'aaaaaaaa-0000-0000-0000-0000000000a1';
    y UUID := 'aaaaaaaa-0000-0000-0000-0000000000b1';
    z UUID := 'aaaaaaaa-0000-0000-0000-0000000000c1';
BEGIN
    ASSERT account_balance_at(x, '2026-01-15') = 1100, 'case4 branch1 pre-anchor';
    ASSERT account_balance_at(x, '2026-02-10') = 5030, 'case4 branch1 post-anchor';
    ASSERT account_balance_at(y, '2026-02-10') = 1940, 'case4 branch2';
    ASSERT account_balance_at(z, 'infinity') = 250, 'case4 branch3';
    ASSERT account_balance_at(z, '2026-01-04') = 0, 'case4 branch3 empty';
    RAISE NOTICE 'case 4 passed';
END $$;

-- Case 5: same-day upsert replaces the first snapshot.
INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000a1', '2026-01-31', 6000, 'simplefin')
ON CONFLICT (account_id, as_of_date) DO UPDATE
   SET balance = EXCLUDED.balance, source = EXCLUDED.source;

DO $$
DECLARE x UUID := 'aaaaaaaa-0000-0000-0000-0000000000a1';
BEGIN
    ASSERT (SELECT COUNT(*) FROM account_balance_snapshots
             WHERE account_id = x AND as_of_date = '2026-01-31') = 1, 'case5 single row';
    ASSERT account_balance_at(x, '2026-01-31') = 6000, 'case5 01-31';
    ASSERT account_balance_at(x, '2026-02-10') = 6030, 'case5 02-10';
    -- Plain insert on the same day must violate UNIQUE.
    BEGIN
        INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
        VALUES ('aaaaaaaa-0000-0000-0000-000000000001', x, '2026-01-31', 1, 'manual');
        RAISE EXCEPTION 'case5: duplicate same-day insert was accepted';
    EXCEPTION WHEN unique_violation THEN NULL;
    END;
    RAISE NOTICE 'case 5 passed';
END $$;

-- Case 6: opening/source CHECK.
DO $$
DECLARE y UUID := 'aaaaaaaa-0000-0000-0000-0000000000b1';
BEGIN
    BEGIN
        INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
        VALUES ('aaaaaaaa-0000-0000-0000-000000000001', y, '-infinity', 1, 'manual');
        RAISE EXCEPTION 'case6: manual at -infinity was accepted';
    EXCEPTION WHEN check_violation THEN NULL;
    END;
    BEGIN
        INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
        VALUES ('aaaaaaaa-0000-0000-0000-000000000001', y, '2026-04-01', 1, 'opening');
        RAISE EXCEPTION 'case6: opening at a real date was accepted';
    EXCEPTION WHEN check_violation THEN NULL;
    END;
    RAISE NOTICE 'case 6 passed';
END $$;

ROLLBACK;

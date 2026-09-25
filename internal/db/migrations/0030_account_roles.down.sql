-- 0030_account_roles.down.sql
--
-- Restores the hand-picked cash list and drops roles.
--
-- Whether a user had picked a list is not recoverable, so the list is rebuilt
-- from roles wherever the old default would count something the roles do not:
-- an asset that is neither checking nor savings and not linked on the
-- Retirement tab. Everyone else gets no list, which is the old default. Either
-- way the rolled-back code arrives at the same cash figure.

BEGIN;

ALTER TABLE wealth_profiles ADD COLUMN override_liquid_account_ids UUID[];

WITH cash AS (
    SELECT a.user_id,
           array_agg(a.id) FILTER (WHERE a.role IN ('checking', 'savings')) AS ids,
           bool_or((a.role IS NULL OR a.role NOT IN ('checking', 'savings'))
                   AND r.account_id IS NULL) AS differs
    FROM accounts a
    LEFT JOIN retirement_account_terms r ON r.account_id = a.id
    WHERE a.type = 'asset'
    GROUP BY a.user_id
)
UPDATE wealth_profiles p SET override_liquid_account_ids = cash.ids
FROM cash
WHERE p.user_id = cash.user_id AND cash.differs AND cash.ids IS NOT NULL;

INSERT INTO wealth_profile_fields (user_id, field_key, source, answered_at)
SELECT user_id, 'override_liquid_account_ids', 'derived', NOW()
FROM wealth_profiles
WHERE cardinality(override_liquid_account_ids) > 0
ON CONFLICT (user_id, field_key) DO NOTHING;

ALTER TABLE accounts DROP CONSTRAINT accounts_role_matches_type;
ALTER TABLE accounts DROP COLUMN role;

COMMIT;

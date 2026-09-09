ALTER TABLE subscriptions DROP CONSTRAINT subscriptions_billing_cycle_check;
ALTER TABLE subscriptions ADD CONSTRAINT subscriptions_billing_cycle_check CHECK (billing_cycle = ANY (ARRAY['monthly'::text, 'yearly'::text]));

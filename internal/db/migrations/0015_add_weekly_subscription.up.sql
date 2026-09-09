ALTER TABLE subscriptions DROP CONSTRAINT subscriptions_billing_cycle_check;
ALTER TABLE subscriptions ADD CONSTRAINT subscriptions_billing_cycle_check CHECK (billing_cycle = ANY (ARRAY['weekly'::text, 'monthly'::text, 'yearly'::text]));

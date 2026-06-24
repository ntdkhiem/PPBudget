ALTER TABLE transactions ADD COLUMN subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL;

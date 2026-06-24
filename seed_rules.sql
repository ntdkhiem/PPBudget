-- Seed Categories
INSERT INTO categories (id, name, type) VALUES ('c0000000-0000-4000-a000-000000000001', 'Food', 'expense') ON CONFLICT DO NOTHING;
INSERT INTO categories (id, name, type) VALUES ('c0000000-0000-4000-a000-000000000002', 'Student Loans', 'expense') ON CONFLICT DO NOTHING;
INSERT INTO categories (id, name, type) VALUES ('c0000000-0000-4000-a000-000000000003', 'Subscription', 'expense') ON CONFLICT DO NOTHING;
INSERT INTO categories (id, name, type) VALUES ('c0000000-0000-4000-a000-000000000004', 'Family Mortgage', 'expense') ON CONFLICT DO NOTHING;

-- Seed Rules
-- 1. Food Rules
INSERT INTO rules (id, name, description, trigger_type, strictness, priority, is_active) VALUES 
('d0000000-0000-4000-a000-000000000001', 'Food - Groceries', 'Matches common grocery stores', 'transaction_created', 'any', 10, true),
('d0000000-0000-4000-a000-000000000002', 'Food - Fast Food', 'Matches fast food chains', 'transaction_created', 'any', 10, true),
('d0000000-0000-4000-a000-000000000003', 'Food - Delivery', 'Matches UberEats, DoorDash, etc.', 'transaction_created', 'any', 10, true);

-- Actions for Food Rules
INSERT INTO rule_actions (id, rule_id, action_type, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000001', 'set_category', 'c0000000-0000-4000-a000-000000000001'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000002', 'set_category', 'c0000000-0000-4000-a000-000000000001'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000003', 'set_category', 'c0000000-0000-4000-a000-000000000001');

-- Conditions for Food - Groceries
INSERT INTO rule_conditions (id, rule_id, field, operator, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000001', 'description', 'contains', 'whole foods'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000001', 'description', 'contains', 'trader joe'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000001', 'description', 'contains', 'kroger'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000001', 'description', 'contains', 'safeway'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000001', 'description', 'contains', 'walmart'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000001', 'description', 'contains', 'costco'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000001', 'description', 'contains', 'target'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000001', 'description', 'contains', 'aldi'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000001', 'description', 'contains', 'publix'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000001', 'description', 'contains', 'wegmans');

-- Conditions for Food - Fast Food
INSERT INTO rule_conditions (id, rule_id, field, operator, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000002', 'description', 'contains', 'mcdonald'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000002', 'description', 'contains', 'burger king'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000002', 'description', 'contains', 'wendy'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000002', 'description', 'contains', 'taco bell'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000002', 'description', 'contains', 'starbucks'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000002', 'description', 'contains', 'dunkin'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000002', 'description', 'contains', 'subway'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000002', 'description', 'contains', 'chipotle'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000002', 'description', 'contains', 'panera'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000002', 'description', 'contains', 'chick-fil-a');

-- Conditions for Food - Delivery
INSERT INTO rule_conditions (id, rule_id, field, operator, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000003', 'description', 'contains', 'ubereats'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000003', 'description', 'contains', 'uber eats'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000003', 'description', 'contains', 'doordash'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000003', 'description', 'contains', 'grubhub'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000003', 'description', 'contains', 'postmates'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000003', 'description', 'contains', 'seamless');

-- 2. Student Loans
INSERT INTO rules (id, name, description, trigger_type, strictness, priority, is_active) VALUES 
('d0000000-0000-4000-a000-000000000004', 'Student Loans', 'Matches DEPT EDUCATION', 'transaction_created', 'any', 10, true);

INSERT INTO rule_actions (id, rule_id, action_type, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000004', 'set_category', 'c0000000-0000-4000-a000-000000000002');

INSERT INTO rule_conditions (id, rule_id, field, operator, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000004', 'description', 'contains', 'dept education');

-- 3. Subscription
INSERT INTO rules (id, name, description, trigger_type, strictness, priority, is_active) VALUES 
('d0000000-0000-4000-a000-000000000005', 'Subscription - Toyota Connected', 'Matches TOYOTACONNECTED', 'transaction_created', 'any', 10, true),
('d0000000-0000-4000-a000-000000000006', 'Subscription - Planet Fitness', 'Matches planet fitness', 'transaction_created', 'any', 10, true),
('d0000000-0000-4000-a000-000000000007', 'Subscription - Google Cloud', 'Matches google cloud', 'transaction_created', 'any', 10, true),
('d0000000-0000-4000-a000-000000000008', 'Subscription - Simplefin Bridge', 'Matches simplefin bridge', 'transaction_created', 'any', 10, true);

INSERT INTO rule_actions (id, rule_id, action_type, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000005', 'set_category', 'c0000000-0000-4000-a000-000000000003'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000006', 'set_category', 'c0000000-0000-4000-a000-000000000003'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000007', 'set_category', 'c0000000-0000-4000-a000-000000000003'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000008', 'set_category', 'c0000000-0000-4000-a000-000000000003');

INSERT INTO rule_conditions (id, rule_id, field, operator, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000005', 'description', 'contains', 'toyotaconnected'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000006', 'description', 'contains', 'planet fitness'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000007', 'description', 'contains', 'google cloud'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000008', 'description', 'contains', 'simplefin bridge');

-- 4. Family Mortgage
INSERT INTO rules (id, name, description, trigger_type, strictness, priority, is_active) VALUES 
('d0000000-0000-4000-a000-000000000009', 'Family Mortgage', 'Matches family mortgage payments', 'transaction_created', 'any', 10, true);

INSERT INTO rule_actions (id, rule_id, action_type, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000009', 'set_category', 'c0000000-0000-4000-a000-000000000004');

INSERT INTO rule_conditions (id, rule_id, field, operator, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000009', 'description', 'contains', 'mortgage'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000009', 'description', 'contains', 'home loan');

-- Insert new categories
INSERT INTO categories (id, name, type) VALUES ('c0000000-0000-4000-a000-000000000005', 'Take out/Dine out', 'expense') ON CONFLICT DO NOTHING;
INSERT INTO categories (id, name, type) VALUES ('c0000000-0000-4000-a000-000000000006', 'Gas + Car Fixes', 'expense') ON CONFLICT DO NOTHING;
INSERT INTO categories (id, name, type) VALUES ('c0000000-0000-4000-a000-000000000007', 'Grocery', 'expense') ON CONFLICT DO NOTHING;
INSERT INTO categories (id, name, type) VALUES ('c0000000-0000-4000-a000-000000000008', 'Shopping', 'expense') ON CONFLICT DO NOTHING;
INSERT INTO categories (id, name, type) VALUES ('c0000000-0000-4000-a000-000000000009', 'PI', 'expense') ON CONFLICT DO NOTHING;
INSERT INTO categories (id, name, type) VALUES ('c0000000-0000-4000-a000-00000000000a', 'Entertainment', 'expense') ON CONFLICT DO NOTHING;

-- Insert rules
INSERT INTO rules (id, name, description, trigger_type, strictness, priority, is_active) VALUES 
('d0000000-0000-4000-a000-000000000010', 'Take out/Dine out Rule', 'Matches restaurants and delivery', 'transaction_created', 'any', 10, true),
('d0000000-0000-4000-a000-000000000011', 'Gas + Car Fixes Rule', 'Matches gas stations and auto shops', 'transaction_created', 'any', 10, true),
('d0000000-0000-4000-a000-000000000012', 'Grocery Rule', 'Matches supermarkets and grocers', 'transaction_created', 'any', 10, true),
('d0000000-0000-4000-a000-000000000013', 'Shopping Rule', 'Matches retail and online shopping', 'transaction_created', 'any', 10, true),
('d0000000-0000-4000-a000-000000000014', 'PI Rule', 'Matches personal items / principal interest', 'transaction_created', 'any', 10, true),
('d0000000-0000-4000-a000-000000000015', 'Entertainment Rule', 'Matches movies, gaming, events', 'transaction_created', 'any', 10, true);

-- Actions
INSERT INTO rule_actions (id, rule_id, action_type, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000010', 'set_category', 'c0000000-0000-4000-a000-000000000005'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000011', 'set_category', 'c0000000-0000-4000-a000-000000000006'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000012', 'set_category', 'c0000000-0000-4000-a000-000000000007'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000013', 'set_category', 'c0000000-0000-4000-a000-000000000008'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000014', 'set_category', 'c0000000-0000-4000-a000-000000000009'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000015', 'set_category', 'c0000000-0000-4000-a000-00000000000a');

-- Conditions for Take out/Dine out
INSERT INTO rule_conditions (id, rule_id, field, operator, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000010', 'description', 'contains', 'doordash'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000010', 'description', 'contains', 'ubereats'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000010', 'description', 'contains', 'mcdonald'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000010', 'description', 'contains', 'starbucks'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000010', 'description', 'contains', 'restaurant'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000010', 'description', 'contains', 'pizza'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000010', 'description', 'contains', 'cafe');

-- Conditions for Gas + Car Fixes
INSERT INTO rule_conditions (id, rule_id, field, operator, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000011', 'description', 'contains', 'shell'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000011', 'description', 'contains', 'chevron'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000011', 'description', 'contains', 'exxon'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000011', 'description', 'contains', 'autozone'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000011', 'description', 'contains', 'jiffy lube'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000011', 'description', 'contains', 'valvoline');

-- Conditions for Grocery
INSERT INTO rule_conditions (id, rule_id, field, operator, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000012', 'description', 'contains', 'whole foods'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000012', 'description', 'contains', 'trader joe'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000012', 'description', 'contains', 'kroger'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000012', 'description', 'contains', 'safeway'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000012', 'description', 'contains', 'aldi'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000012', 'description', 'contains', 'publix');

-- Conditions for Shopping
INSERT INTO rule_conditions (id, rule_id, field, operator, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000013', 'description', 'contains', 'amazon'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000013', 'description', 'contains', 'amzn mktp'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000013', 'description', 'contains', 'target'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000013', 'description', 'contains', 'walmart'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000013', 'description', 'contains', 'best buy');

-- Conditions for PI
INSERT INTO rule_conditions (id, rule_id, field, operator, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000014', 'description', 'contains', 'cvs'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000014', 'description', 'contains', 'walgreens'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000014', 'description', 'contains', 'insurance'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000014', 'description', 'contains', 'hair salon');

-- Conditions for Entertainment
INSERT INTO rule_conditions (id, rule_id, field, operator, value) VALUES
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000015', 'description', 'contains', 'netflix'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000015', 'description', 'contains', 'hulu'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000015', 'description', 'contains', 'steam'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000015', 'description', 'contains', 'amc'),
(gen_random_uuid(), 'd0000000-0000-4000-a000-000000000015', 'description', 'contains', 'ticketmaster');

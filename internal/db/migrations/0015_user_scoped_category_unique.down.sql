ALTER TABLE categories DROP CONSTRAINT IF EXISTS categories_user_id_name_key;

-- We have to manually handle any duplicates that might have been introduced 
-- before we can restore the original unique constraint.
-- For simplicity in a down migration, we just add the constraint, 
-- but this will fail if duplicates actually exist.
ALTER TABLE categories ADD CONSTRAINT categories_name_key UNIQUE (name);

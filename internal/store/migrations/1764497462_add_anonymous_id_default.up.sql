ALTER TABLE users 
  ALTER COLUMN anonymous_id SET DEFAULT uuid_generate_v4()::text;

CREATE OR REPLACE function increment_version() RETURNS trigger
  LANGUAGE plpgsql
  AS $$
BEGIN
  new.version := old.version + 1;
  return new;
END;
$$;

CREATE TRIGGER increment_version_users BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE PROCEDURE increment_version();
CREATE TRIGGER increment_version_lists BEFORE UPDATE ON lists
  FOR EACH ROW EXECUTE PROCEDURE increment_version();
CREATE TRIGGER increment_version_user_list BEFORE UPDATE ON user_list
  FOR EACH ROW EXECUTE PROCEDURE increment_version();

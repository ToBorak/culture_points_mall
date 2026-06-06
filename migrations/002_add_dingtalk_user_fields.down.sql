ALTER TABLE users
  DROP INDEX uk_tenant_union,
  DROP COLUMN union_id,
  DROP COLUMN is_admin;

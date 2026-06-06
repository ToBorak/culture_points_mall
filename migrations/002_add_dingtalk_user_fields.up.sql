-- 钉钉用户扩展字段：unionId（日历接口需要）+ 是否管理员（RBAC）
ALTER TABLE users
  ADD COLUMN union_id VARCHAR(128) DEFAULT NULL,
  ADD COLUMN is_admin TINYINT(1) NOT NULL DEFAULT 0,
  ADD UNIQUE KEY uk_tenant_union (tenant_id, union_id);

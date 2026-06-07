-- 价值观维度
CREATE TABLE value_dimensions (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  code VARCHAR(32) NOT NULL,
  name VARCHAR(64) NOT NULL,
  keywords VARCHAR(255) DEFAULT '',
  weight DECIMAL(3,2) DEFAULT 1.00,
  sort_order INT DEFAULT 0,
  enabled TINYINT DEFAULT 1,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_tenant_code (tenant_id, code)
) ENGINE=InnoDB CHARSET=utf8mb4;

-- 租户
CREATE TABLE tenants (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  ding_corp_id VARCHAR(64) DEFAULT NULL,
  config_json JSON DEFAULT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_corp (ding_corp_id)
) ENGINE=InnoDB CHARSET=utf8mb4;

-- 用户
CREATE TABLE users (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  ding_user_id VARCHAR(64) DEFAULT NULL,
  name VARCHAR(64) NOT NULL,
  avatar_url VARCHAR(255) DEFAULT '',
  dept_id BIGINT DEFAULT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_tenant_ding (tenant_id, ding_user_id),
  KEY idx_tenant_dept (tenant_id, dept_id)
) ENGINE=InnoDB CHARSET=utf8mb4;

-- 部门
CREATE TABLE departments (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  name VARCHAR(64) NOT NULL,
  KEY idx_tenant (tenant_id)
) ENGINE=InnoDB CHARSET=utf8mb4;

-- 积分流水
CREATE TABLE point_transactions (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  dimension_id BIGINT NOT NULL,
  amount INT NOT NULL,
  activity_id BIGINT DEFAULT NULL,
  reason VARCHAR(255) DEFAULT '',
  operator_id BIGINT DEFAULT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  KEY idx_user_dim (user_id, dimension_id),
  KEY idx_tenant_dim_time (tenant_id, dimension_id, created_at)
) ENGINE=InnoDB CHARSET=utf8mb4;

-- 用户维度快照
CREATE TABLE user_dimension_scores (
  user_id BIGINT NOT NULL,
  tenant_id BIGINT NOT NULL,
  dimension_id BIGINT NOT NULL,
  total_score INT DEFAULT 0,
  quarter_score INT DEFAULT 0,
  year_score INT DEFAULT 0,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, dimension_id),
  KEY idx_tenant_dim (tenant_id, dimension_id, total_score)
) ENGINE=InnoDB CHARSET=utf8mb4;

-- 活动
CREATE TABLE activities (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  dimension_id BIGINT NOT NULL,
  title VARCHAR(128) NOT NULL,
  status ENUM('draft','published','running','closed') NOT NULL DEFAULT 'draft',
  capacity INT DEFAULT NULL,
  start_at TIMESTAMP NULL,
  end_at TIMESTAMP NULL,
  points_reward INT DEFAULT 0,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  KEY idx_tenant_status (tenant_id, status, start_at)
) ENGINE=InnoDB CHARSET=utf8mb4;

CREATE TABLE activity_enrollments (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  activity_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  status ENUM('enrolled','checked_in','absent') NOT NULL DEFAULT 'enrolled',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_act_user (activity_id, user_id)
) ENGINE=InnoDB CHARSET=utf8mb4;

-- 签到二维码
CREATE TABLE signin_codes (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  activity_id BIGINT NOT NULL,
  code VARCHAR(64) NOT NULL,
  issued_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  expires_at TIMESTAMP NOT NULL,
  KEY idx_act_exp (activity_id, expires_at)
) ENGINE=InnoDB CHARSET=utf8mb4;

CREATE TABLE signin_records (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  activity_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  result ENUM('passed','rejected','suspect') NOT NULL,
  reason VARCHAR(255) DEFAULT '',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  KEY idx_user_act (user_id, activity_id)
) ENGINE=InnoDB CHARSET=utf8mb4;

-- 徽章
CREATE TABLE badges (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  dimension_id BIGINT NOT NULL,
  name VARCHAR(64) NOT NULL,
  rarity ENUM('common','rare','epic','legendary') NOT NULL,
  rule_json JSON DEFAULT NULL,
  icon_url VARCHAR(255) DEFAULT '',
  KEY idx_tenant_dim (tenant_id, dimension_id)
) ENGINE=InnoDB CHARSET=utf8mb4;

CREATE TABLE user_badges (
  user_id BIGINT NOT NULL,
  badge_id BIGINT NOT NULL,
  celebrated TINYINT NOT NULL DEFAULT 0,
  earned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, badge_id)
) ENGINE=InnoDB CHARSET=utf8mb4;

-- 商品
CREATE TABLE mall_items (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  type ENUM('item','blindbox') NOT NULL,
  name VARCHAR(128) NOT NULL,
  cost INT NOT NULL,
  stock INT DEFAULT NULL,
  image_url VARCHAR(255) DEFAULT '',
  KEY idx_tenant_type (tenant_id, type)
) ENGINE=InnoDB CHARSET=utf8mb4;

CREATE TABLE mall_blindbox_pool (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  box_item_id BIGINT NOT NULL,
  prize_name VARCHAR(128) NOT NULL,
  prize_image VARCHAR(255) DEFAULT '',
  weight INT NOT NULL,
  stock INT DEFAULT NULL,
  KEY idx_box (box_item_id)
) ENGINE=InnoDB CHARSET=utf8mb4;

CREATE TABLE mall_blindbox_freeze (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tx_id VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL,
  box_item_id BIGINT NOT NULL,
  amount INT NOT NULL,
  status ENUM('try','confirmed','cancelled') NOT NULL DEFAULT 'try',
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_tx (tx_id),
  KEY idx_status_exp (status, expires_at)
) ENGINE=InnoDB CHARSET=utf8mb4;

CREATE TABLE mall_orders (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  item_id BIGINT DEFAULT NULL,
  prize_id BIGINT DEFAULT NULL,
  cost INT NOT NULL,
  status ENUM('paid','shipped','done','cancelled') NOT NULL DEFAULT 'paid',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  KEY idx_user (user_id)
) ENGINE=InnoDB CHARSET=utf8mb4;

-- Agent 会话
CREATE TABLE agent_sessions (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  operator_id BIGINT NOT NULL,
  title VARCHAR(128) DEFAULT '',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  KEY idx_tenant_op (tenant_id, operator_id)
) ENGINE=InnoDB CHARSET=utf8mb4;

CREATE TABLE agent_messages (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  session_id BIGINT NOT NULL,
  role ENUM('user','assistant','tool','system') NOT NULL,
  content JSON NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  KEY idx_session (session_id, created_at)
) ENGINE=InnoDB CHARSET=utf8mb4;

-- 钉钉 Mock 出库
CREATE TABLE dingtalk_mock_outbox (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  api VARCHAR(64) NOT NULL,
  target VARCHAR(255) DEFAULT '',
  payload JSON NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  KEY idx_tenant_time (tenant_id, created_at)
) ENGINE=InnoDB CHARSET=utf8mb4;

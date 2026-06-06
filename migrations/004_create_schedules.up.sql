-- 日程发布记录（一次发布 = 一行，记录建日历/推群的目标与结果）
CREATE TABLE schedules (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  title VARCHAR(128) NOT NULL,
  start_at TIMESTAMP NOT NULL,
  end_at TIMESTAMP NOT NULL,
  location VARCHAR(255) DEFAULT '',
  detail VARCHAR(1024) DEFAULT '',
  attendee_user_ids JSON,
  group_ids JSON,
  push_calendar TINYINT(1) NOT NULL DEFAULT 0,
  push_group TINYINT(1) NOT NULL DEFAULT 0,
  status ENUM('published','partial','failed') NOT NULL DEFAULT 'published',
  calendar_event_id VARCHAR(128) DEFAULT '',
  result_note VARCHAR(1024) DEFAULT '',
  created_by BIGINT DEFAULT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  KEY idx_tenant_time (tenant_id, created_at)
) ENGINE=InnoDB CHARSET=utf8mb4;

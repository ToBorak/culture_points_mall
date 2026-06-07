CREATE TABLE publications (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  season_id BIGINT NULL,
  title VARCHAR(128) NOT NULL,
  period_code VARCHAR(16) NOT NULL,
  cover_image_url VARCHAR(255) NULL,
  intro_text TEXT NULL,
  period_start TIMESTAMP NULL,
  period_end TIMESTAMP NULL,
  status ENUM('draft','published','archived') NOT NULL DEFAULT 'draft',
  published_at TIMESTAMP NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_tenant_status (tenant_id, status),
  KEY idx_tenant_period (tenant_id, period_code)
) ENGINE=InnoDB CHARSET=utf8mb4;

CREATE TABLE publication_sections (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  publication_id BIGINT NOT NULL,
  type ENUM('editorial','star','values','honors','lottery','activity','leaderboard','innovation','custom') NOT NULL,
  title VARCHAR(64) NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  visible TINYINT(1) NOT NULL DEFAULT 1,
  ai_copy TEXT NULL,
  config_json JSON NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  KEY idx_pub (publication_id, sort_order)
) ENGINE=InnoDB CHARSET=utf8mb4;

CREATE TABLE publication_articles (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  publication_id BIGINT NULL,
  section_id BIGINT NULL,
  title VARCHAR(128) NOT NULL,
  summary VARCHAR(255) NULL,
  content_html TEXT NOT NULL,
  cover_image_url VARCHAR(255) NULL,
  source_type ENUM('manual','from_nomination') NOT NULL DEFAULT 'manual',
  source_id BIGINT NULL,
  value_dimension_id BIGINT NULL,
  author_id BIGINT NULL,
  status ENUM('draft','published') NOT NULL DEFAULT 'draft',
  published_at TIMESTAMP NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_pub_section (publication_id, section_id),
  KEY idx_tenant_status (tenant_id, status)
) ENGINE=InnoDB CHARSET=utf8mb4;

CREATE TABLE publication_snapshots (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  publication_id BIGINT NOT NULL,
  section_id BIGINT NOT NULL,
  data_json JSON NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_pub_section (publication_id, section_id)
) ENGINE=InnoDB CHARSET=utf8mb4;

CREATE TABLE star_seasons (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  name VARCHAR(64) NOT NULL,
  quarter_code VARCHAR(16) NOT NULL,
  status ENUM('nominating','judging','published','closed') NOT NULL DEFAULT 'nominating',
  nominate_start_at TIMESTAMP NULL,
  nominate_end_at TIMESTAMP NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_tenant_quarter (tenant_id, quarter_code),
  KEY idx_tenant_status (tenant_id, status)
) ENGINE=InnoDB CHARSET=utf8mb4;

CREATE TABLE star_nominations (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  season_id BIGINT NOT NULL,
  nominator_id BIGINT NOT NULL,
  nominee_id BIGINT NOT NULL,
  dimension_id BIGINT NOT NULL,
  case_text TEXT NOT NULL,
  case_refined TEXT NULL,
  ai_tags JSON NULL,
  status ENUM('submitted','duplicate','shortlisted','selected','rejected') NOT NULL DEFAULT 'submitted',
  score DECIMAL(4,1) NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_dedup (season_id, nominator_id, nominee_id, dimension_id),
  KEY idx_season_nominee (season_id, nominee_id),
  KEY idx_season_status (season_id, status),
  KEY idx_nominator_month (tenant_id, nominator_id, created_at),
  KEY idx_nominee_month (tenant_id, nominee_id, created_at)
) ENGINE=InnoDB CHARSET=utf8mb4;

CREATE TABLE star_winners (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  season_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  dimension_id BIGINT NOT NULL,
  citation TEXT NULL,
  source_nomination_id BIGINT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_season_user_dim (season_id, user_id, dimension_id),
  KEY idx_tenant_season (tenant_id, season_id)
) ENGINE=InnoDB CHARSET=utf8mb4;

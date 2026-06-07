package migrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// migrationsTable 记录已应用的迁移版本，使 Up() 幂等。version = 迁移文件名。
const migrationsTable = "schema_migrations"

// alreadyExistsCodes 是「对象已存在 / 重复」类 MySQL 错误码。
// 首次为老库（已有表但无版本表）接入版本追踪时，逐条容忍这些错误并视为已生效，
// 从而把旧库平滑纳入追踪、避免重复 DDL 中断迁移。
//
//	1050 表已存在 / 1060 列重复 / 1061 索引重复 / 1091 待删除的列或键不存在
var alreadyExistsCodes = map[uint16]struct{}{
	1050: {},
	1060: {},
	1061: {},
	1091: {},
}

type Runner struct {
	DB  *gorm.DB
	Dir string
}

// Up 按文件名顺序应用 migrations 目录下尚未记录的 *.up.sql，并把每个版本写入
// schema_migrations，因而可安全重复执行：已应用的跳过、未应用的执行后记录。
func (r *Runner) Up() error {
	// 1) 版本追踪表（幂等建表）。
	if err := r.DB.Exec(
		"CREATE TABLE IF NOT EXISTS " + migrationsTable + " (" +
			"version VARCHAR(128) PRIMARY KEY, " +
			"applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP" +
			") ENGINE=InnoDB CHARSET=utf8mb4",
	).Error; err != nil {
		return fmt.Errorf("create %s: %w", migrationsTable, err)
	}

	// 2) 已应用版本集合。
	applied, err := r.appliedVersions()
	if err != nil {
		return err
	}

	// 3) 老库首次接入判定：版本表为空、但库里已有业务表。此时逐条 apply 容忍
	//    "对象已存在" 类错误（视为既有结构），把旧库纳入追踪而不中断。
	//    全新库（无任何业务表）则严格执行，任何 DDL 错误都暴露。
	adopting := false
	if len(applied) == 0 {
		if adopting, err = r.hasExistingTables(); err != nil {
			return err
		}
	}

	files, err := filepath.Glob(filepath.Join(r.Dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, f := range files {
		version := filepath.Base(f)
		if _, done := applied[version]; done {
			fmt.Println("skip (already applied):", version)
			continue
		}

		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		tolerated, err := r.applyFile(version, string(raw), adopting)
		if err != nil {
			return err
		}

		// 4) 执行成功（或接入老库时视为已生效）后记录版本。
		if err := r.DB.Exec(
			"INSERT INTO "+migrationsTable+" (version) VALUES (?)", version,
		).Error; err != nil {
			return fmt.Errorf("record %s: %w", version, err)
		}
		if tolerated {
			fmt.Println("adopted (already present):", version)
		} else {
			fmt.Println("applied:", version)
		}
	}
	return nil
}

// applyFile 执行单个迁移文件的全部语句。adopting=true（老库首次接入）时改用静默
// logger 执行，并容忍"对象已存在"类错误，避免重复建表把日志刷红；返回是否发生过
// 容忍，用于区分 applied / adopted 输出。
func (r *Runner) applyFile(version, raw string, adopting bool) (tolerated bool, err error) {
	db := r.DB
	if adopting {
		db = r.DB.Session(&gorm.Session{Logger: r.DB.Logger.LogMode(logger.Silent)})
	}
	for _, stmt := range splitSQL(raw) {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if e := db.Exec(stmt).Error; e != nil {
			if adopting && isAlreadyExists(e) {
				tolerated = true
				continue
			}
			return tolerated, fmt.Errorf("apply %s: %w", version, e)
		}
	}
	return tolerated, nil
}

// appliedVersions 读出 schema_migrations 中已记录的版本集合。
func (r *Runner) appliedVersions() (map[string]struct{}, error) {
	var versions []string
	if err := r.DB.Raw("SELECT version FROM " + migrationsTable).Scan(&versions).Error; err != nil {
		return nil, fmt.Errorf("load applied versions: %w", err)
	}
	set := make(map[string]struct{}, len(versions))
	for _, v := range versions {
		set[v] = struct{}{}
	}
	return set, nil
}

// hasExistingTables 判断当前库里是否已有除版本表以外的业务表。
func (r *Runner) hasExistingTables() (bool, error) {
	var count int64
	if err := r.DB.Raw(
		"SELECT COUNT(*) FROM information_schema.tables "+
			"WHERE table_schema = DATABASE() AND table_name <> ?",
		migrationsTable,
	).Scan(&count).Error; err != nil {
		return false, fmt.Errorf("inspect existing tables: %w", err)
	}
	return count > 0, nil
}

// isAlreadyExists 判断 err 是否为"对象已存在 / 重复"类 MySQL 错误。
func isAlreadyExists(err error) bool {
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) {
		_, ok := alreadyExistsCodes[myErr.Number]
		return ok
	}
	return false
}

func splitSQL(raw string) []string {
	return strings.Split(raw, ";\n")
}

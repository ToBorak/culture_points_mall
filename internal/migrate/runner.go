package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type Runner struct {
	DB  *gorm.DB
	Dir string
}

func (r *Runner) Up() error {
	files, err := filepath.Glob(filepath.Join(r.Dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		for _, stmt := range splitSQL(string(raw)) {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if err := r.DB.Exec(stmt).Error; err != nil {
				return fmt.Errorf("apply %s: %w", filepath.Base(f), err)
			}
		}
		fmt.Println("applied:", filepath.Base(f))
	}
	return nil
}

func splitSQL(raw string) []string {
	return strings.Split(raw, ";\n")
}

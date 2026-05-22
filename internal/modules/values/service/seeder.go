package service

import (
	"context"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/standardsoftware/culture_points_mall/internal/modules/values/domain"
)

type dimensionDef struct {
	Code      string  `yaml:"code"`
	Name      string  `yaml:"name"`
	Keywords  string  `yaml:"keywords"`
	Weight    float64 `yaml:"weight"`
	SortOrder int     `yaml:"sort_order"`
}

type seedFile struct {
	Dimensions []dimensionDef `yaml:"dimensions"`
}

func (s *Service) SeedDefaults(ctx context.Context, tenantID int64, yamlPath string) error {
	raw, err := os.ReadFile(yamlPath)
	if err != nil {
		return err
	}
	var f seedFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return err
	}
	for _, d := range f.Dimensions {
		if err := s.repo.Upsert(ctx, &domain.Dimension{
			TenantID: tenantID, Code: d.Code, Name: d.Name, Keywords: d.Keywords,
			Weight: d.Weight, SortOrder: d.SortOrder, Enabled: true,
		}); err != nil {
			return err
		}
	}
	s.invalidate()
	return nil
}

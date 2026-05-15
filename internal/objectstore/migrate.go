package objectstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type MigrationResult struct {
	Uploaded int
	Skipped  int
	Failed   int
	Errors   []string
}

func UploadDirectory(ctx context.Context, cfg Config, root string) MigrationResult {
	cfg = cfg.Normalize()
	result := MigrationResult{}
	if !cfg.Ready() {
		result.Failed = 1
		result.Errors = append(result.Errors, "R2 storage is not configured")
		return result
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		result.Failed = 1
		result.Errors = append(result.Errors, err.Error())
		return result
	}
	_ = filepath.WalkDir(rootAbs, func(filePath string, d os.DirEntry, err error) error {
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err.Error())
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(rootAbs, filePath)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err.Error())
			return nil
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(filepath.Base(rel), ".") {
			result.Skipped++
			return nil
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err.Error())
			return nil
		}
		uploadCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		_, err = Upload(uploadCtx, cfg, cfg.ObjectKey(rel), data, ContentTypeForPath(filePath))
		cancel()
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, rel+": "+err.Error())
			return nil
		}
		result.Uploaded++
		return nil
	})
	return result
}

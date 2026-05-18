package service

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"chatgpt2api/internal/objectstore"
	"chatgpt2api/internal/storage"
)

const (
	ThumbnailSize         = 480
	thumbnailQuality      = 72
	thumbnailCacheVersion = 3
	thumbnailExtension    = ".jpg"

	ImageVisibilityPrivate = "private"
	ImageVisibilityPublic  = "public"
)

type ImageConfig interface {
	ImagesDir() string
	ImageThumbnailsDir() string
	ImageMetadataDir() string
	CleanupOldImages() int
}

type imageObjectStorageConfig interface {
	ImageObjectStorage() objectstore.Config
}

type ImageAccessScope struct {
	OwnerID string
	All     bool
	Public  bool
}

type imageMetadata struct {
	OwnerID          string
	OwnerName        string
	Visibility       string
	PublishedAt      string
	ResolutionPreset string
	RequestedSize    string
	OutputFormat     string
}

type GeneratedImageMetadata struct {
	ResolutionPreset string
	RequestedSize    string
	OutputFormat     string
}

type RemoteImageRecord struct {
	Path             string
	Name             string
	Date             string
	Size             int64
	URL              string
	Storage          string
	ObjectKey        string
	CreatedAt        string
	Visibility       string
	OwnerID          string
	OwnerName        string
	PublishedAt      string
	ResolutionPreset string
	RequestedSize    string
	OutputFormat     string
	Width            int
	Height           int
}

type ImageFileAccess struct {
	Rel        string
	Path       string
	Info       os.FileInfo
	Visibility string
	OwnerID    string
}

type ImageService struct {
	config        ImageConfig
	store         storage.JSONDocumentBackend
	thumbnailMu   sync.Mutex
	thumbnailJobs map[string]*thumbnailJob
}

type imageFileRef struct {
	rel  string
	path string
	info os.FileInfo
}

type thumbnailJob struct {
	done   chan struct{}
	result map[string]any
}

func NewImageService(config ImageConfig, backend ...storage.Backend) *ImageService {
	return &ImageService{config: config, store: firstJSONDocumentStore(backend)}
}

func (s *ImageService) ListImages(baseURL, startDate, endDate string, scope ImageAccessScope) map[string]any {
	s.config.CleanupOldImages()
	root := s.config.ImagesDir()
	items := make([]map[string]any, 0)
	localPaths := map[string]struct{}{}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		localPaths[rel] = struct{}{}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		parts := strings.Split(rel, "/")
		day := info.ModTime().Format("2006-01-02")
		if len(parts) >= 4 {
			day = strings.Join(parts[:3], "-")
		}
		if startDate != "" && day < startDate {
			return nil
		}
		if endDate != "" && day > endDate {
			return nil
		}
		meta := s.imageMetadata(rel)
		ownerID := meta.OwnerID
		if scope.Public {
			if meta.Visibility != ImageVisibilityPublic {
				return nil
			}
		} else if !scope.All && (scope.OwnerID == "" || ownerID != scope.OwnerID) {
			return nil
		}
		thumb := s.thumbnailInfo(rel, info)
		item := map[string]any{
			"name":       filepath.Base(path),
			"path":       rel,
			"date":       day,
			"size":       info.Size(),
			"url":        publicAssetURL(baseURL, "images", rel),
			"created_at": info.ModTime().Format("2006-01-02 15:04:05"),
			"visibility": meta.Visibility,
		}
		if ownerID != "" {
			item["owner_id"] = ownerID
		}
		if meta.OwnerName != "" {
			item["owner_name"] = meta.OwnerName
		}
		if meta.PublishedAt != "" {
			item["published_at"] = meta.PublishedAt
		}
		if meta.ResolutionPreset != "" {
			item["resolution_preset"] = meta.ResolutionPreset
		}
		if meta.RequestedSize != "" {
			item["requested_size"] = meta.RequestedSize
		}
		if meta.OutputFormat != "" {
			item["output_format"] = meta.OutputFormat
		}
		if provider, ok := s.config.(imageObjectStorageConfig); ok {
			cfg := provider.ImageObjectStorage()
			if cfg.PublicBaseURL != "" {
				item["r2_url"] = cfg.PublicURL(cfg.ObjectKey(rel))
			}
		}
		if thumbRel, ok := thumb["thumbnail_rel"].(string); ok && thumbRel != "" {
			item["thumbnail_url"] = thumbnailURL(baseURL, thumbRel, info.ModTime())
		} else {
			item["thumbnail_url"] = ""
		}
		if !setImageItemDimensions(item, thumb["width"], thumb["height"]) {
			if width, height, ok := imageFileDimensions(path); ok {
				setImageItemDimensions(item, width, height)
			}
		}
		items = append(items, item)
		return nil
	})
	items = append(items, s.remoteImageItems(startDate, endDate, scope, localPaths)...)
	sort.Slice(items, func(i, j int) bool {
		left := toString(items[i]["created_at"])
		right := toString(items[j]["created_at"])
		if scope.Public {
			left = firstNonEmptyString(toString(items[i]["published_at"]), left)
			right = firstNonEmptyString(toString(items[j]["published_at"]), right)
		}
		return strings.Compare(left, right) > 0
	})
	groupMap := map[string][]map[string]any{}
	var order []string
	for _, item := range items {
		day := toString(item["date"])
		if _, ok := groupMap[day]; !ok {
			order = append(order, day)
		}
		groupMap[day] = append(groupMap[day], item)
	}
	groups := make([]map[string]any, 0, len(order))
	for _, day := range order {
		groups = append(groups, map[string]any{"date": day, "items": groupMap[day]})
	}
	return map[string]any{"items": items, "groups": groups}
}

func (s *ImageService) ListImagesPage(baseURL, startDate, endDate string, scope ImageAccessScope, page, pageSize int) map[string]any {
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}
	if page < 1 {
		page = 1
	}
	all := s.ListImages(baseURL, startDate, endDate, scope)
	allItems, _ := all["items"].([]map[string]any)
	if allItems == nil {
		allItems = []map[string]any{}
	}
	total := len(allItems)
	totalPages := 1
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items := allItems[start:end]
	groupMap := map[string][]map[string]any{}
	order := make([]string, 0)
	for _, item := range items {
		day := toString(item["date"])
		if _, ok := groupMap[day]; !ok {
			order = append(order, day)
		}
		groupMap[day] = append(groupMap[day], item)
	}
	groups := make([]map[string]any, 0, len(order))
	for _, day := range order {
		groups = append(groups, map[string]any{"date": day, "items": groupMap[day]})
	}
	return map[string]any{
		"items":      items,
		"groups":     groups,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
		"total_page": totalPages,
	}
}

func (s *ImageService) remoteImageItems(startDate, endDate string, scope ImageAccessScope, localPaths map[string]struct{}) []map[string]any {
	records := LoadRemoteImageRecords(s.store, s.config.ImagesDir())
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		if record.Path == "" || record.URL == "" {
			continue
		}
		if _, ok := localPaths[record.Path]; ok {
			continue
		}
		day := record.Date
		if day == "" {
			day = imageDay(record.Path, time.Now())
		}
		if startDate != "" && day < startDate {
			continue
		}
		if endDate != "" && day > endDate {
			continue
		}
		meta := imageMetadata{
			OwnerID:          record.OwnerID,
			OwnerName:        record.OwnerName,
			Visibility:       record.Visibility,
			PublishedAt:      record.PublishedAt,
			ResolutionPreset: record.ResolutionPreset,
			RequestedSize:    record.RequestedSize,
			OutputFormat:     record.OutputFormat,
		}
		if meta.Visibility == "" {
			meta.Visibility = ImageVisibilityPrivate
		}
		if !imageMetadataAllowsAccess(meta, scope) {
			continue
		}
		item := map[string]any{
			"name":          firstNonEmptyString(record.Name, filepath.Base(record.Path)),
			"path":          record.Path,
			"date":          day,
			"size":          record.Size,
			"url":           record.URL,
			"r2_url":        record.URL,
			"thumbnail_url": record.URL,
			"created_at":    firstNonEmptyString(record.CreatedAt, day+" 00:00:00"),
			"visibility":    meta.Visibility,
			"storage":       firstNonEmptyString(record.Storage, "r2"),
		}
		if meta.OwnerID != "" {
			item["owner_id"] = meta.OwnerID
		}
		if meta.OwnerName != "" {
			item["owner_name"] = meta.OwnerName
		}
		if meta.PublishedAt != "" {
			item["published_at"] = meta.PublishedAt
		}
		if meta.ResolutionPreset != "" {
			item["resolution_preset"] = meta.ResolutionPreset
		}
		if meta.RequestedSize != "" {
			item["requested_size"] = meta.RequestedSize
		}
		if meta.OutputFormat != "" {
			item["output_format"] = meta.OutputFormat
		}
		if record.Width > 0 && record.Height > 0 {
			setImageItemDimensions(item, record.Width, record.Height)
		}
		items = append(items, item)
	}
	return items
}

func (s *ImageService) UpdateImageVisibility(value, visibility string, scope ImageAccessScope) (map[string]any, error) {
	visibility, err := NormalizeImageVisibility(visibility)
	if err != nil {
		return nil, err
	}
	rel, err := imageRelativePathFromValue(value)
	if err != nil {
		return nil, err
	}
	imageRoot, err := filepath.Abs(s.config.ImagesDir())
	if err != nil {
		return nil, err
	}
	ref, err := s.imageFileRef(imageRoot, rel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("image not found")
		}
		return nil, err
	}
	meta := s.imageMetadata(ref.rel)
	if !scope.All && (scope.OwnerID == "" || meta.OwnerID != scope.OwnerID) {
		return nil, errors.New("image not found")
	}
	if err := s.writeImageMetadataForRef(ref, "", "", visibility); err != nil {
		return nil, err
	}
	nextMeta := s.imageMetadata(ref.rel)
	item := map[string]any{
		"name":       filepath.Base(ref.path),
		"path":       ref.rel,
		"date":       imageDay(ref.rel, ref.info.ModTime()),
		"size":       ref.info.Size(),
		"visibility": nextMeta.Visibility,
		"created_at": ref.info.ModTime().Format("2006-01-02 15:04:05"),
	}
	if nextMeta.OwnerID != "" {
		item["owner_id"] = nextMeta.OwnerID
	}
	if nextMeta.OwnerName != "" {
		item["owner_name"] = nextMeta.OwnerName
	}
	if nextMeta.PublishedAt != "" {
		item["published_at"] = nextMeta.PublishedAt
	}
	if nextMeta.ResolutionPreset != "" {
		item["resolution_preset"] = nextMeta.ResolutionPreset
	}
	if nextMeta.RequestedSize != "" {
		item["requested_size"] = nextMeta.RequestedSize
	}
	if width, height, ok := imageFileDimensions(ref.path); ok {
		setImageItemDimensions(item, width, height)
	}
	return item, nil
}

func (s *ImageService) ImageFileAccess(value string, scope ImageAccessScope) (ImageFileAccess, error) {
	rel, err := imageRelativePathFromValue(value)
	if err != nil {
		return ImageFileAccess{}, err
	}
	imageRoot, err := filepath.Abs(s.config.ImagesDir())
	if err != nil {
		return ImageFileAccess{}, err
	}
	ref, err := s.imageFileRef(imageRoot, rel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ImageFileAccess{}, errors.New("image not found")
		}
		return ImageFileAccess{}, err
	}
	meta := s.imageMetadata(ref.rel)
	if !imageMetadataAllowsAccess(meta, scope) {
		return ImageFileAccess{}, errors.New("image not found")
	}
	return ImageFileAccess{
		Rel:        ref.rel,
		Path:       ref.path,
		Info:       ref.info,
		Visibility: meta.Visibility,
		OwnerID:    meta.OwnerID,
	}, nil
}

func (s *ImageService) DeleteImages(paths []string, scope ImageAccessScope) (map[string]any, error) {
	if len(paths) == 0 {
		return nil, errors.New("paths is required")
	}
	imageRoot, err := filepath.Abs(s.config.ImagesDir())
	if err != nil {
		return nil, err
	}
	thumbnailRoot, err := filepath.Abs(s.config.ImageThumbnailsDir())
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(paths))
	deleted := 0
	missing := 0
	removedPaths := make([]string, 0, len(paths))
	for _, value := range paths {
		rel, err := cleanImageRelativePath(value)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		remoteRecord, hasRemoteRecord := s.remoteImageRecord(rel)

		imagePath := filepath.Join(imageRoot, filepath.FromSlash(rel))
		if !pathInsideRoot(imageRoot, imagePath) {
			return nil, errors.New("invalid image path")
		}
		ownerID := s.imageOwner(rel)
		if ownerID == "" {
			ownerID = remoteRecord.OwnerID
		}
		if !scope.All && (scope.OwnerID == "" || ownerID != scope.OwnerID) {
			missing++
			continue
		}
		if err := s.removeImageThumbnail(thumbnailRoot, rel); err != nil {
			return nil, err
		}
		if err := s.removeImageOwner(rel); err != nil {
			return nil, err
		}
		if hasRemoteRecord {
			_ = RemoveRemoteImageRecords(s.store, s.config.ImagesDir(), []string{rel})
		}
		info, err := os.Stat(imagePath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			if hasRemoteRecord {
				deleted++
			} else {
				missing++
			}
		} else if info.IsDir() {
			return nil, errors.New("image path is not a file")
		} else if err := os.Remove(imagePath); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			missing++
		} else {
			deleted++
		}

		removeEmptyParentDirs(imageRoot, filepath.Dir(imagePath))
		removedPaths = append(removedPaths, rel)
	}
	return map[string]any{"deleted": deleted, "missing": missing, "paths": removedPaths}, nil
}

func (s *ImageService) remoteImageRecord(rel string) (RemoteImageRecord, bool) {
	for _, record := range LoadRemoteImageRecords(s.store, s.config.ImagesDir()) {
		if record.Path == rel {
			return record, true
		}
	}
	return RemoteImageRecord{}, false
}

func (s *ImageService) UploadImagesToObjectStorage(ctx context.Context, paths []string, scope ImageAccessScope) (map[string]any, error) {
	if len(paths) == 0 {
		return nil, errors.New("paths is required")
	}
	provider, ok := s.config.(imageObjectStorageConfig)
	if !ok {
		return nil, errors.New("R2 storage is not supported")
	}
	cfg := provider.ImageObjectStorage()
	if !cfg.Ready() {
		return nil, errors.New("R2 storage is not configured")
	}
	imageRoot, err := filepath.Abs(s.config.ImagesDir())
	if err != nil {
		return nil, err
	}
	thumbnailRoot, err := filepath.Abs(s.config.ImageThumbnailsDir())
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(paths))
	items := make([]map[string]any, 0, len(paths))
	var errorsList []map[string]any
	uploaded := 0
	skipped := 0
	missing := 0
	failed := 0
	for _, value := range paths {
		rel, err := cleanImageRelativePath(value)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		remoteRecord, hasRemoteRecord := s.remoteImageRecord(rel)
		ref, err := s.imageFileRef(imageRoot, rel)
		if err != nil {
			if hasRemoteRecord && strings.TrimSpace(remoteRecord.URL) != "" {
				meta := imageMetadata{
					OwnerID:          remoteRecord.OwnerID,
					OwnerName:        remoteRecord.OwnerName,
					Visibility:       remoteRecord.Visibility,
					PublishedAt:      remoteRecord.PublishedAt,
					ResolutionPreset: remoteRecord.ResolutionPreset,
					RequestedSize:    remoteRecord.RequestedSize,
					OutputFormat:     remoteRecord.OutputFormat,
				}
				if meta.Visibility == "" {
					meta.Visibility = ImageVisibilityPrivate
				}
				if !imageMetadataAllowsAccess(meta, scope) {
					missing++
					continue
				}
				skipped++
				items = append(items, map[string]any{"path": rel, "key": firstNonEmptyString(remoteRecord.ObjectKey, cfg.ObjectKey(rel)), "url": remoteRecord.URL, "skipped": true})
				continue
			}
			missing++
			continue
		}
		meta := s.imageMetadata(ref.rel)
		if !imageMetadataAllowsAccess(meta, scope) {
			missing++
			continue
		}
		if hasRemoteRecord && strings.TrimSpace(remoteRecord.URL) != "" {
			if removeErr := os.Remove(ref.path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				failed++
				errorsList = append(errorsList, map[string]any{"path": ref.rel, "error": removeErr.Error()})
				continue
			}
			_ = s.removeImageThumbnail(thumbnailRoot, ref.rel)
			_ = s.removeImageOwner(ref.rel)
			removeEmptyParentDirs(imageRoot, filepath.Dir(ref.path))
			skipped++
			items = append(items, map[string]any{"path": ref.rel, "key": firstNonEmptyString(remoteRecord.ObjectKey, cfg.ObjectKey(ref.rel)), "url": remoteRecord.URL, "skipped": true})
			continue
		}
		data, err := os.ReadFile(ref.path)
		if err != nil {
			failed++
			errorsList = append(errorsList, map[string]any{"path": ref.rel, "error": err.Error()})
			continue
		}
		width, height, _ := imageFileDimensions(ref.path)
		key := cfg.ObjectKey(ref.rel)
		publicURL, err := objectstore.Upload(ctx, cfg, key, data, objectstore.ContentTypeForPath(ref.path))
		if err != nil {
			failed++
			errorsList = append(errorsList, map[string]any{"path": ref.rel, "error": err.Error()})
			continue
		}
		if publicURL == "" {
			failed++
			errorsList = append(errorsList, map[string]any{"path": ref.rel, "error": "R2 public base URL is required before deleting local image"})
			continue
		}
		createdAt := ref.info.ModTime().Format("2006-01-02 15:04:05")
		if saveErr := SaveRemoteImageRecord(s.store, s.config.ImagesDir(), RemoteImageRecord{
			Path:             ref.rel,
			Name:             filepath.Base(ref.path),
			Date:             imageDay(ref.rel, ref.info.ModTime()),
			Size:             ref.info.Size(),
			URL:              publicURL,
			Storage:          "r2",
			ObjectKey:        key,
			CreatedAt:        createdAt,
			Visibility:       meta.Visibility,
			OwnerID:          meta.OwnerID,
			OwnerName:        meta.OwnerName,
			PublishedAt:      meta.PublishedAt,
			ResolutionPreset: meta.ResolutionPreset,
			RequestedSize:    meta.RequestedSize,
			OutputFormat:     meta.OutputFormat,
			Width:            width,
			Height:           height,
		}); saveErr != nil {
			failed++
			errorsList = append(errorsList, map[string]any{"path": ref.rel, "error": saveErr.Error()})
			continue
		}
		if removeErr := os.Remove(ref.path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			failed++
			errorsList = append(errorsList, map[string]any{"path": ref.rel, "error": removeErr.Error()})
			continue
		}
		_ = s.removeImageThumbnail(thumbnailRoot, ref.rel)
		_ = s.removeImageOwner(ref.rel)
		removeEmptyParentDirs(imageRoot, filepath.Dir(ref.path))
		uploaded++
		item := map[string]any{"path": ref.rel, "key": key}
		if publicURL != "" {
			item["url"] = publicURL
		}
		items = append(items, item)
	}
	return map[string]any{
		"uploaded": uploaded,
		"skipped":  skipped,
		"missing":  missing,
		"failed":   failed,
		"items":    items,
		"errors":   errorsList,
	}, nil
}

func (s *ImageService) RecordImageOwners(values []string, ownerID string) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return
	}
	for _, ref := range s.imageFileRefs(values) {
		_ = s.writeImageMetadataForRef(ref, ownerID, "", "")
	}
}

func (s *ImageService) RecordGeneratedImages(values []string, ownerID, ownerName, visibility string, metadataValues ...GeneratedImageMetadata) {
	ownerID = strings.TrimSpace(ownerID)
	ownerName = strings.TrimSpace(ownerName)
	metadata := GeneratedImageMetadata{}
	if len(metadataValues) > 0 {
		metadata = metadataValues[0]
	}
	visibility, err := NormalizeImageVisibility(visibility)
	if err != nil {
		visibility = ImageVisibilityPrivate
	}
	for _, ref := range s.imageFileRefs(values) {
		s.ensureThumbnailForRef(ref)
		if ownerID != "" && ownerID != "anonymous" {
			_ = s.writeImageMetadataForRef(ref, ownerID, ownerName, visibility, metadata)
		}
	}
	for _, value := range values {
		rel, err := imageRelativePathFromValue(value)
		if err != nil {
			continue
		}
		record, ok := s.remoteImageRecord(rel)
		if !ok {
			continue
		}
		if ownerID != "" && ownerID != "anonymous" {
			record.OwnerID = ownerID
			record.OwnerName = ownerName
		}
		record.Visibility = visibility
		if visibility == ImageVisibilityPublic && record.PublishedAt == "" {
			record.PublishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		}
		if visibility != ImageVisibilityPublic {
			record.PublishedAt = ""
		}
		if preset := NormalizeImageResolutionPreset(metadata.ResolutionPreset); preset != "" {
			record.ResolutionPreset = preset
		}
		if requestedSize := strings.TrimSpace(metadata.RequestedSize); requestedSize != "" {
			record.RequestedSize = requestedSize
		}
		if outputFormat := NormalizeImageOutputFormat(metadata.OutputFormat); outputFormat != "" {
			record.OutputFormat = outputFormat
		}
		_ = SaveRemoteImageRecord(s.store, s.config.ImagesDir(), record)
	}
}

func (s *ImageService) EnsureThumbnails(values []string) {
	for _, ref := range s.imageFileRefs(values) {
		s.ensureThumbnailForRef(ref)
	}
}

func (s *ImageService) SourceImageRelativePathFromThumbnail(thumbnailRel string) (string, error) {
	return sourceImageRelativePathFromThumbnail(thumbnailRel)
}

func (s *ImageService) EnsureThumbnail(thumbnailRel string) error {
	sourceRel, err := s.SourceImageRelativePathFromThumbnail(thumbnailRel)
	if err != nil {
		return err
	}
	imageRoot, err := filepath.Abs(s.config.ImagesDir())
	if err != nil {
		return err
	}
	ref, err := s.imageFileRef(imageRoot, sourceRel)
	if err != nil {
		return err
	}
	thumb := s.ensureThumbnailForRef(ref)
	if toString(thumb["thumbnail_rel"]) == "" {
		return errors.New("thumbnail unavailable")
	}
	return nil
}

func (s *ImageService) thumbnailInfo(rel string, sourceInfo os.FileInfo) map[string]any {
	_, result, _ := s.thumbnailCacheInfo(rel, sourceInfo.ModTime())
	return result
}

func (s *ImageService) ensureThumbnailForRef(ref imageFileRef) map[string]any {
	if _, result, ok := s.thumbnailCacheInfo(ref.rel, ref.info.ModTime()); ok {
		return result
	}
	return s.withThumbnailJob(ref.rel, func() map[string]any {
		if _, result, ok := s.thumbnailCacheInfo(ref.rel, ref.info.ModTime()); ok {
			return result
		}
		return s.generateThumbnail(ref)
	})
}

func (s *ImageService) withThumbnailJob(rel string, run func() map[string]any) map[string]any {
	s.thumbnailMu.Lock()
	if s.thumbnailJobs == nil {
		s.thumbnailJobs = make(map[string]*thumbnailJob)
	}
	if job, ok := s.thumbnailJobs[rel]; ok {
		done := job.done
		s.thumbnailMu.Unlock()
		<-done
		return job.result
	}
	job := &thumbnailJob{done: make(chan struct{})}
	s.thumbnailJobs[rel] = job
	s.thumbnailMu.Unlock()

	job.result = run()

	s.thumbnailMu.Lock()
	delete(s.thumbnailJobs, rel)
	close(job.done)
	s.thumbnailMu.Unlock()
	return job.result
}

func (s *ImageService) thumbnailCacheInfo(rel string, sourceModTime time.Time) (string, map[string]any, bool) {
	thumbPath := s.thumbnailPath(rel)
	thumbRel := thumbnailRelativePath(s.config.ImageThumbnailsDir(), thumbPath)
	result := map[string]any{"thumbnail_rel": thumbRel}
	thumbInfo, err := os.Stat(thumbPath)
	if err != nil || thumbInfo.ModTime().Before(sourceModTime) {
		return thumbPath, result, false
	}
	meta := s.readThumbnailMetadata(rel, thumbPath+".json", sourceModTime)
	if !isCurrentThumbnailMetadata(meta) {
		return thumbPath, result, false
	}
	for key, value := range meta {
		result[key] = value
	}
	return thumbPath, result, true
}

func (s *ImageService) generateThumbnail(ref imageFileRef) map[string]any {
	thumbPath, result, _ := s.thumbnailCacheInfo(ref.rel, ref.info.ModTime())
	file, err := os.Open(ref.path)
	if err != nil {
		return map[string]any{}
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return map[string]any{}
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	thumb := resizeToFit(flattenImage(img), ThumbnailSize, ThumbnailSize)
	if err := writeJPEGThumbnail(thumbPath, thumb); err != nil {
		return map[string]any{}
	}
	_ = s.writeThumbnailMetadata(ref.rel, thumbPath+".json", map[string]any{
		"width":             width,
		"height":            height,
		"thumbnail_format":  "jpeg",
		"thumbnail_quality": thumbnailQuality,
		"thumbnail_size":    ThumbnailSize,
		"thumbnail_version": thumbnailCacheVersion,
	})
	result["width"] = width
	result["height"] = height
	return result
}

func (s *ImageService) imageFileRefs(values []string) []imageFileRef {
	if len(values) == 0 {
		return nil
	}
	imageRoot, err := filepath.Abs(s.config.ImagesDir())
	if err != nil {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	refs := make([]imageFileRef, 0, len(values))
	for _, value := range values {
		rel, err := imageRelativePathFromValue(value)
		if err != nil {
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		ref, err := s.imageFileRef(imageRoot, rel)
		if err != nil {
			continue
		}
		refs = append(refs, ref)
	}
	return refs
}

func (s *ImageService) imageFileRef(imageRoot, rel string) (imageFileRef, error) {
	rel, err := cleanImageRelativePath(rel)
	if err != nil {
		return imageFileRef{}, err
	}
	imagePath := filepath.Join(imageRoot, filepath.FromSlash(rel))
	if !pathInsideRoot(imageRoot, imagePath) {
		return imageFileRef{}, errors.New("invalid image path")
	}
	info, err := os.Stat(imagePath)
	if err != nil {
		return imageFileRef{}, err
	}
	if info.IsDir() {
		return imageFileRef{}, errors.New("image path is not a file")
	}
	return imageFileRef{rel: rel, path: imagePath, info: info}, nil
}

func (s *ImageService) thumbnailPath(rel string) string {
	return filepath.Join(s.config.ImageThumbnailsDir(), filepath.FromSlash(rel)+thumbnailExtension)
}

func (s *ImageService) imageOwner(rel string) string {
	return s.imageMetadata(rel).OwnerID
}

func imageMetadataAllowsAccess(meta imageMetadata, scope ImageAccessScope) bool {
	if meta.Visibility == ImageVisibilityPublic {
		return true
	}
	if scope.All {
		return true
	}
	return scope.OwnerID != "" && meta.OwnerID == scope.OwnerID
}

func (s *ImageService) imageMetadata(rel string) imageMetadata {
	metaPath, err := s.imageOwnerMetadataPath(rel)
	if err != nil {
		return imageMetadata{Visibility: ImageVisibilityPrivate}
	}
	var raw map[string]any
	if s.store != nil {
		value, err := s.store.LoadJSONDocument(imageOwnerDocumentName(rel))
		if err == nil {
			if meta, ok := value.(map[string]any); ok {
				raw = meta
			}
		}
	}
	if raw == nil {
		data, err := os.ReadFile(metaPath)
		if err != nil {
			return imageMetadata{Visibility: ImageVisibilityPrivate}
		}
		if json.Unmarshal(data, &raw) != nil {
			return imageMetadata{Visibility: ImageVisibilityPrivate}
		}
	}
	return normalizeImageMetadata(raw)
}

func normalizeImageMetadata(raw map[string]any) imageMetadata {
	visibility := strings.TrimSpace(toString(raw["visibility"]))
	if visibility != ImageVisibilityPublic {
		visibility = ImageVisibilityPrivate
	}
	return imageMetadata{
		OwnerID:          strings.TrimSpace(toString(raw["owner_id"])),
		OwnerName:        strings.TrimSpace(toString(raw["owner_name"])),
		Visibility:       visibility,
		PublishedAt:      strings.TrimSpace(toString(raw["published_at"])),
		ResolutionPreset: NormalizeImageResolutionPreset(toString(raw["resolution_preset"])),
		RequestedSize:    strings.TrimSpace(toString(raw["requested_size"])),
		OutputFormat:     NormalizeImageOutputFormat(strings.TrimSpace(toString(raw["output_format"]))),
	}
}

func (s *ImageService) writeImageMetadataForRef(ref imageFileRef, ownerID, ownerName, visibility string, metadataValues ...GeneratedImageMetadata) error {
	meta := s.imageMetadata(ref.rel)
	if ownerID = strings.TrimSpace(ownerID); ownerID != "" {
		meta.OwnerID = ownerID
	}
	if ownerName = strings.TrimSpace(ownerName); ownerName != "" {
		meta.OwnerName = ownerName
	}
	if visibility = strings.TrimSpace(visibility); visibility != "" {
		normalized, err := NormalizeImageVisibility(visibility)
		if err != nil {
			return err
		}
		if normalized == ImageVisibilityPublic {
			if meta.PublishedAt == "" || meta.Visibility != ImageVisibilityPublic {
				meta.PublishedAt = time.Now().UTC().Format(time.RFC3339Nano)
			}
		} else {
			meta.PublishedAt = ""
		}
		meta.Visibility = normalized
	}
	if len(metadataValues) > 0 {
		metadata := metadataValues[0]
		if preset := NormalizeImageResolutionPreset(metadata.ResolutionPreset); preset != "" {
			meta.ResolutionPreset = preset
		}
		if requestedSize := strings.TrimSpace(metadata.RequestedSize); requestedSize != "" {
			meta.RequestedSize = requestedSize
		}
		if outputFormat := NormalizeImageOutputFormat(metadata.OutputFormat); outputFormat != "" {
			meta.OutputFormat = outputFormat
		}
	}
	if meta.Visibility == "" {
		meta.Visibility = ImageVisibilityPrivate
	}
	return s.writeImageMetadata(ref.rel, meta)
}

func (s *ImageService) writeImageMetadata(rel string, meta imageMetadata) error {
	metaPath, err := s.imageOwnerMetadataPath(rel)
	if err != nil {
		return err
	}
	value := map[string]any{
		"visibility": meta.Visibility,
		"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	if meta.OwnerID != "" {
		value["owner_id"] = meta.OwnerID
	}
	if meta.OwnerName != "" {
		value["owner_name"] = meta.OwnerName
	}
	if meta.PublishedAt != "" {
		value["published_at"] = meta.PublishedAt
	}
	if meta.ResolutionPreset != "" {
		value["resolution_preset"] = meta.ResolutionPreset
	}
	if meta.RequestedSize != "" {
		value["requested_size"] = meta.RequestedSize
	}
	if meta.OutputFormat != "" {
		value["output_format"] = meta.OutputFormat
	}
	if s.store != nil {
		return s.store.SaveJSONDocument(imageOwnerDocumentName(rel), value)
	}
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		return err
	}
	return writeJSONFile(metaPath, value)
}

func (s *ImageService) removeImageOwner(rel string) error {
	metaPath, err := s.imageOwnerMetadataPath(rel)
	if err != nil {
		return err
	}
	if s.store != nil {
		return s.store.DeleteJSONDocument(imageOwnerDocumentName(rel))
	}
	removeErr := os.Remove(metaPath)
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return removeErr
	}
	removeEmptyParentDirs(s.config.ImageMetadataDir(), filepath.Dir(metaPath))
	return nil
}

func (s *ImageService) imageOwnerMetadataPath(rel string) (string, error) {
	rel, err := cleanImageRelativePath(rel)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(s.config.ImageMetadataDir())
	if err != nil {
		return "", err
	}
	metaPath := filepath.Join(root, filepath.FromSlash(rel)+".json")
	if !pathInsideRoot(root, metaPath) {
		return "", errors.New("invalid image path")
	}
	return metaPath, nil
}

func (s *ImageService) readThumbnailMetadata(rel, metaPath string, sourceMtime time.Time) map[string]any {
	if s.store != nil {
		raw, err := s.store.LoadJSONDocument(thumbnailMetadataDocumentName(rel))
		if err == nil {
			if meta, ok := raw.(map[string]any); ok && meta["width"] != nil && meta["height"] != nil {
				return meta
			}
		}
	}
	return readImageMetadata(metaPath, sourceMtime)
}

func (s *ImageService) writeThumbnailMetadata(rel, metaPath string, value map[string]any) error {
	if s.store != nil {
		return s.store.SaveJSONDocument(thumbnailMetadataDocumentName(rel), value)
	}
	return writeJSONFile(metaPath, value)
}

func (s *ImageService) removeImageThumbnail(root, rel string) error {
	if s.store != nil {
		if err := s.store.DeleteJSONDocument(thumbnailMetadataDocumentName(rel)); err != nil {
			return err
		}
	}
	return removeImageThumbnail(root, rel)
}

const remoteImageIndexDocumentName = "image_remote_index.json"

func LoadRemoteImageRecords(store storage.JSONDocumentBackend, imagesDir string) []RemoteImageRecord {
	raw := loadStoredJSON(store, remoteImageIndexDocumentName, remoteImageIndexPath(imagesDir))
	items := utilMapSlice(raw)
	records := make([]RemoteImageRecord, 0, len(items))
	for _, item := range items {
		record := remoteImageRecordFromMap(item)
		if record.Path != "" && record.URL != "" {
			records = append(records, record)
		}
	}
	return records
}

func SaveRemoteImageRecord(store storage.JSONDocumentBackend, imagesDir string, record RemoteImageRecord) error {
	record.Path = filepath.ToSlash(strings.TrimSpace(record.Path))
	record.URL = strings.TrimSpace(record.URL)
	if record.Path == "" || record.URL == "" {
		return nil
	}
	records := LoadRemoteImageRecords(store, imagesDir)
	next := make([]RemoteImageRecord, 0, len(records)+1)
	replaced := false
	for _, item := range records {
		if item.Path == record.Path {
			next = append(next, record)
			replaced = true
			continue
		}
		next = append(next, item)
	}
	if !replaced {
		next = append(next, record)
	}
	return saveStoredJSON(store, remoteImageIndexDocumentName, remoteImageIndexPath(imagesDir), remoteImageRecordsToMaps(next))
}

func RemoveRemoteImageRecords(store storage.JSONDocumentBackend, imagesDir string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	remove := map[string]struct{}{}
	for _, item := range paths {
		if rel := filepath.ToSlash(strings.TrimSpace(item)); rel != "" {
			remove[rel] = struct{}{}
		}
	}
	records := LoadRemoteImageRecords(store, imagesDir)
	next := make([]RemoteImageRecord, 0, len(records))
	for _, record := range records {
		if _, ok := remove[record.Path]; ok {
			continue
		}
		next = append(next, record)
	}
	return saveStoredJSON(store, remoteImageIndexDocumentName, remoteImageIndexPath(imagesDir), remoteImageRecordsToMaps(next))
}

func remoteImageIndexPath(imagesDir string) string {
	return filepath.Join(filepath.Dir(imagesDir), remoteImageIndexDocumentName)
}

func remoteImageRecordsToMaps(records []RemoteImageRecord) []map[string]any {
	out := make([]map[string]any, 0, len(records))
	for _, record := range records {
		item := map[string]any{
			"path":       record.Path,
			"name":       record.Name,
			"date":       record.Date,
			"size":       record.Size,
			"url":        record.URL,
			"object_key": record.ObjectKey,
			"created_at": record.CreatedAt,
			"visibility": record.Visibility,
			"width":      record.Width,
			"height":     record.Height,
		}
		if record.OwnerID != "" {
			item["owner_id"] = record.OwnerID
		}
		if record.OwnerName != "" {
			item["owner_name"] = record.OwnerName
		}
		if record.PublishedAt != "" {
			item["published_at"] = record.PublishedAt
		}
		if record.ResolutionPreset != "" {
			item["resolution_preset"] = record.ResolutionPreset
		}
		if record.RequestedSize != "" {
			item["requested_size"] = record.RequestedSize
		}
		if record.OutputFormat != "" {
			item["output_format"] = record.OutputFormat
		}
		out = append(out, item)
	}
	return out
}

func remoteImageRecordFromMap(item map[string]any) RemoteImageRecord {
	return RemoteImageRecord{
		Path:             strings.TrimSpace(toString(item["path"])),
		Name:             strings.TrimSpace(toString(item["name"])),
		Date:             strings.TrimSpace(toString(item["date"])),
		Size:             int64(numericMetaValue(item["size"])),
		URL:              strings.TrimSpace(toString(item["url"])),
		ObjectKey:        strings.TrimSpace(toString(item["object_key"])),
		CreatedAt:        strings.TrimSpace(toString(item["created_at"])),
		Visibility:       strings.TrimSpace(toString(item["visibility"])),
		OwnerID:          strings.TrimSpace(toString(item["owner_id"])),
		OwnerName:        strings.TrimSpace(toString(item["owner_name"])),
		PublishedAt:      strings.TrimSpace(toString(item["published_at"])),
		ResolutionPreset: NormalizeImageResolutionPreset(toString(item["resolution_preset"])),
		RequestedSize:    strings.TrimSpace(toString(item["requested_size"])),
		OutputFormat:     NormalizeImageOutputFormat(toString(item["output_format"])),
		Width:            numericMetaValue(item["width"]),
		Height:           numericMetaValue(item["height"]),
	}
}

func utilMapSlice(raw any) []map[string]any {
	switch list := raw.(type) {
	case []map[string]any:
		return list
	case []any:
		out := make([]map[string]any, 0, len(list))
		for _, item := range list {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		return utilMapSlice(list["items"])
	default:
		return nil
	}
}

func imageOwnerDocumentName(rel string) string {
	return "image_metadata/" + filepath.ToSlash(rel) + ".json"
}

func NormalizeImageVisibility(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "", ImageVisibilityPrivate:
		return ImageVisibilityPrivate, nil
	case ImageVisibilityPublic:
		return ImageVisibilityPublic, nil
	default:
		return "", errors.New("visibility must be private or public")
	}
}

func NormalizeImageResolutionPreset(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1k", "1080p":
		return "1k"
	case "2k":
		return "2k"
	case "4k":
		return "4k"
	default:
		return ""
	}
}

func imageDay(rel string, modTime time.Time) string {
	parts := strings.Split(rel, "/")
	if len(parts) >= 4 {
		return strings.Join(parts[:3], "-")
	}
	return modTime.Format("2006-01-02")
}

func thumbnailMetadataDocumentName(rel string) string {
	return "image_thumbnails/" + filepath.ToSlash(rel) + thumbnailExtension + ".json"
}

func sourceImageRelativePathFromThumbnail(value string) (string, error) {
	thumbnailRel, err := cleanImageRelativePath(value)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(thumbnailRel, thumbnailExtension) {
		return "", errors.New("invalid thumbnail path")
	}
	return cleanImageRelativePath(strings.TrimSuffix(thumbnailRel, thumbnailExtension))
}

func setImageItemDimensions(item map[string]any, widthValue, heightValue any) bool {
	width, height, ok := imageDimensionsFromValues(widthValue, heightValue)
	if !ok {
		return false
	}
	item["width"] = width
	item["height"] = height
	item["resolution"] = strconv.Itoa(width) + "x" + strconv.Itoa(height)
	item["aspect_ratio"] = simplifiedAspectRatio(width, height)
	item["orientation"] = imageOrientation(width, height)
	item["megapixels"] = float64(width) * float64(height) / 1_000_000
	return true
}

func imageDimensionsFromValues(widthValue, heightValue any) (int, int, bool) {
	width := numericMetaValue(widthValue)
	height := numericMetaValue(heightValue)
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func imageFileDimensions(path string) (int, int, bool) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return 0, 0, false
	}
	return config.Width, config.Height, true
}

func simplifiedAspectRatio(width, height int) string {
	divisor := greatestCommonDivisor(width, height)
	if divisor <= 0 {
		return ""
	}
	return strconv.Itoa(width/divisor) + ":" + strconv.Itoa(height/divisor)
}

func imageOrientation(width, height int) string {
	if width == height {
		return "square"
	}
	if width > height {
		return "landscape"
	}
	return "portrait"
}

func greatestCommonDivisor(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func thumbnailRelativePath(root, thumbPath string) string {
	rel, err := filepath.Rel(root, thumbPath)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}

func publicAssetURL(baseURL, prefix, rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.Trim(prefix, "/") + "/" + strings.Join(parts, "/")
}

func thumbnailURL(baseURL, thumbRel string, sourceModTime time.Time) string {
	return publicAssetURL(baseURL, "image-thumbnails", thumbRel) +
		"?v=" + strconv.Itoa(thumbnailCacheVersion) + "-" + strconv.FormatInt(sourceModTime.UnixNano(), 10)
}

func cleanImageRelativePath(value string) (string, error) {
	rel := filepath.ToSlash(strings.TrimSpace(value))
	if rel == "" || strings.ContainsRune(rel, 0) || strings.HasPrefix(rel, "/") || filepath.IsAbs(filepath.FromSlash(rel)) {
		return "", errors.New("invalid image path")
	}
	if path.Clean(rel) != rel {
		return "", errors.New("invalid image path")
	}
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." || part == ".." || strings.Contains(part, ":") {
			return "", errors.New("invalid image path")
		}
	}
	return rel, nil
}

func imageRelativePathFromValue(value string) (string, error) {
	text := strings.TrimSpace(value)
	if text == "" {
		return "", errors.New("invalid image path")
	}
	if parsed, err := url.Parse(text); err == nil {
		pathValue := parsed.EscapedPath()
		if pathValue == "" {
			pathValue = parsed.Path
		}
		if parsed.Scheme != "" || strings.HasPrefix(pathValue, "/") {
			const imagePrefix = "/images/"
			index := strings.Index(pathValue, imagePrefix)
			if index < 0 {
				return "", errors.New("invalid image path")
			}
			rel, err := url.PathUnescape(pathValue[index+len(imagePrefix):])
			if err != nil {
				return "", errors.New("invalid image path")
			}
			return cleanImageRelativePath(rel)
		}
	}
	return cleanImageRelativePath(text)
}

func removeImageThumbnail(root, rel string) error {
	thumbPath := filepath.Join(root, filepath.FromSlash(rel)+thumbnailExtension)
	if !pathInsideRoot(root, thumbPath) {
		return errors.New("invalid image path")
	}
	removeErr := os.Remove(thumbPath)
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return removeErr
	}
	metaErr := os.Remove(thumbPath + ".json")
	if metaErr != nil && !errors.Is(metaErr, os.ErrNotExist) {
		return metaErr
	}
	removeEmptyParentDirs(root, filepath.Dir(thumbPath))
	return nil
}

func writeJPEGThumbnail(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	encodeErr := jpeg.Encode(tmp, img, &jpeg.Options{Quality: thumbnailQuality})
	closeErr := tmp.Close()
	if encodeErr != nil || closeErr != nil {
		_ = os.Remove(tmpPath)
		if encodeErr != nil {
			return encodeErr
		}
		return closeErr
	}
	if err := os.Rename(tmpPath, path); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			_ = os.Remove(tmpPath)
			return err
		}
		if renameErr := os.Rename(tmpPath, path); renameErr != nil {
			_ = os.Remove(tmpPath)
			return renameErr
		}
	}
	return nil
}

func pathInsideRoot(root, target string) bool {
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, targetAbs)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func removeEmptyParentDirs(root, start string) {
	current, err := filepath.Abs(start)
	if err != nil {
		return
	}
	for pathInsideRoot(root, current) {
		err := os.Remove(current)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return
		}
		current = filepath.Dir(current)
	}
}

func readImageMetadata(path string, sourceMtime time.Time) map[string]any {
	info, err := os.Stat(path)
	if err != nil || info.ModTime().Before(sourceMtime) {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var meta map[string]any
	if json.Unmarshal(data, &meta) != nil {
		return nil
	}
	if meta["width"] == nil || meta["height"] == nil {
		return nil
	}
	return meta
}

func isCurrentThumbnailMetadata(meta map[string]any) bool {
	return numericMetaValue(meta["thumbnail_version"]) == thumbnailCacheVersion &&
		numericMetaValue(meta["thumbnail_size"]) == ThumbnailSize &&
		numericMetaValue(meta["thumbnail_quality"]) == thumbnailQuality
}

func numericMetaValue(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return int(n)
		}
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return n
		}
	default:
		return 0
	}
	return 0
}

func flattenImage(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, b, src, b.Min, draw.Over)
	return dst
}

func resizeToFit(src image.Image, maxW, maxH int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	scale := float64(maxW) / float64(w)
	if sh := float64(maxH) / float64(h); sh < scale {
		scale = sh
	}
	if scale > 1 {
		scale = 1
	}
	nw, nh := int(float64(w)*scale), int(float64(h)*scale)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		fy := (float64(y)+0.5)*float64(h)/float64(nh) - 0.5
		y0 := int(fy)
		dy := fy - float64(y0)
		if y0 < 0 {
			y0 = 0
			dy = 0
		}
		y1 := y0 + 1
		if y1 >= h {
			y1 = h - 1
		}
		for x := 0; x < nw; x++ {
			fx := (float64(x)+0.5)*float64(w)/float64(nw) - 0.5
			x0 := int(fx)
			dx := fx - float64(x0)
			if x0 < 0 {
				x0 = 0
				dx = 0
			}
			x1 := x0 + 1
			if x1 >= w {
				x1 = w - 1
			}
			dst.Set(x, y, bilinearColor(
				src.At(b.Min.X+x0, b.Min.Y+y0),
				src.At(b.Min.X+x1, b.Min.Y+y0),
				src.At(b.Min.X+x0, b.Min.Y+y1),
				src.At(b.Min.X+x1, b.Min.Y+y1),
				dx,
				dy,
			))
		}
	}
	return dst
}

func bilinearColor(c00, c10, c01, c11 color.Color, dx, dy float64) color.RGBA {
	r00, g00, b00, a00 := c00.RGBA()
	r10, g10, b10, a10 := c10.RGBA()
	r01, g01, b01, a01 := c01.RGBA()
	r11, g11, b11, a11 := c11.RGBA()
	return color.RGBA{
		R: uint8(bilinearChannel(r00, r10, r01, r11, dx, dy) >> 8),
		G: uint8(bilinearChannel(g00, g10, g01, g11, dx, dy) >> 8),
		B: uint8(bilinearChannel(b00, b10, b01, b11, dx, dy) >> 8),
		A: uint8(bilinearChannel(a00, a10, a01, a11, dx, dy) >> 8),
	}
}

func bilinearChannel(c00, c10, c01, c11 uint32, dx, dy float64) uint32 {
	top := float64(c00)*(1-dx) + float64(c10)*dx
	bottom := float64(c01)*(1-dx) + float64(c11)*dx
	return uint32(top*(1-dy) + bottom*dy + 0.5)
}

func writeJSONFile(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

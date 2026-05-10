package service

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"chatgpt2api/internal/storage"
	"chatgpt2api/internal/util"
)

const (
	LogTypeCall    = "call"
	LogTypeAccount = "account"
)

type LogService struct {
	mu    sync.Mutex
	path  string
	store storage.LogBackend
}

type userUsageDay struct {
	Calls     int
	Success   int
	Failure   int
	QuotaUsed int
}

type userUsageAccumulator struct {
	Calls     int
	Success   int
	Failure   int
	QuotaUsed int
	Daily     map[string]*userUsageDay
}

func NewLogService(dataDir string, backend ...storage.Backend) *LogService {
	path := filepath.Join(dataDir, "logs.jsonl")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	return &LogService{path: path, store: firstLogStore(backend)}
}

func (s *LogService) Add(logType, summary string, detail map[string]any) {
	if detail == nil {
		detail = map[string]any{}
	}
	item := map[string]any{
		"time":    util.NowLocal(),
		"type":    logType,
		"summary": summary,
		"detail":  detail,
	}
	if s.store != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		_ = s.store.AppendLog(item)
		return
	}
	data, err := json.Marshal(item)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.Write(append(data, '\n'))
}

func (s *LogService) List(logType, startDate, endDate string, limit int) []map[string]any {
	if limit <= 0 {
		limit = 200
	}
	if s.store != nil {
		items, err := s.store.QueryLogs(logType, startDate, endDate, limit)
		if err == nil {
			return items
		}
	}
	file, err := os.Open(s.path)
	if err != nil {
		return []map[string]any{}
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	out := make([]map[string]any, 0, limit)
	for i := len(lines) - 1; i >= 0 && len(out) < limit; i-- {
		var item map[string]any
		if json.Unmarshal([]byte(lines[i]), &item) != nil {
			continue
		}
		t := util.Clean(item["time"])
		day := t
		if len(day) > 10 {
			day = day[:10]
		}
		if logType != "" && item["type"] != logType {
			continue
		}
		if startDate != "" && day < startDate {
			continue
		}
		if endDate != "" && day > endDate {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (s *LogService) UserUsageStats(days int) map[string]map[string]any {
	dates := usageDates(days)
	out := map[string]map[string]any{}
	if len(dates) == 0 {
		return out
	}
	startDate := dates[0]
	endDate := dates[len(dates)-1]
	byUser := map[string]*userUsageAccumulator{}
	if s.store != nil {
		items, err := s.store.QueryLogs(LogTypeCall, startDate, endDate, 0)
		if err == nil {
			for _, item := range items {
				accumulateUserUsageLog(byUser, item, startDate, endDate)
			}
			for userID, acc := range byUser {
				out[userID] = userUsageStatsMap(acc, dates)
			}
			return out
		}
	}
	file, err := os.Open(s.path)
	if err != nil {
		return out
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var item map[string]any
		if json.Unmarshal([]byte(scanner.Text()), &item) != nil || item["type"] != LogTypeCall {
			continue
		}
		accumulateUserUsageLog(byUser, item, startDate, endDate)
	}
	for userID, acc := range byUser {
		out[userID] = userUsageStatsMap(acc, dates)
	}
	return out
}

func accumulateUserUsageLog(byUser map[string]*userUsageAccumulator, item map[string]any, startDate, endDate string) {
	if item["type"] != LogTypeCall {
		return
	}
	day := logDay(item)
	if day == "" || day < startDate || day > endDate {
		return
	}
	detail := util.StringMap(item["detail"])
	userID := util.Clean(detail["subject_id"])
	if userID == "" {
		userID = util.Clean(detail["key_id"])
	}
	if userID == "" {
		return
	}
	acc := byUser[userID]
	if acc == nil {
		acc = newUserUsageAccumulator()
		byUser[userID] = acc
	}
	status := util.Clean(detail["status"])
	quotaUsed := logQuotaUsed(detail, status)
	acc.Calls++
	acc.QuotaUsed += quotaUsed
	if status == "success" {
		acc.Success++
	} else if status == "failed" {
		acc.Failure++
	}
	daily := acc.Daily[day]
	if daily == nil {
		daily = &userUsageDay{}
		acc.Daily[day] = daily
	}
	daily.Calls++
	daily.QuotaUsed += quotaUsed
	if status == "success" {
		daily.Success++
	} else if status == "failed" {
		daily.Failure++
	}
}

func ZeroUserUsageStats(days int) map[string]any {
	return userUsageStatsMap(newUserUsageAccumulator(), usageDates(days))
}

func usageDates(days int) []string {
	if days <= 0 {
		days = 14
	}
	if days > 90 {
		days = 90
	}
	start := time.Now().AddDate(0, 0, -days+1)
	dates := make([]string, 0, days)
	for i := 0; i < days; i++ {
		dates = append(dates, start.AddDate(0, 0, i).Format("2006-01-02"))
	}
	return dates
}

func newUserUsageAccumulator() *userUsageAccumulator {
	return &userUsageAccumulator{Daily: map[string]*userUsageDay{}}
}

func userUsageStatsMap(acc *userUsageAccumulator, dates []string) map[string]any {
	if acc == nil {
		acc = newUserUsageAccumulator()
	}
	curve := make([]map[string]any, 0, len(dates))
	for _, date := range dates {
		day := acc.Daily[date]
		if day == nil {
			day = &userUsageDay{}
		}
		curve = append(curve, map[string]any{
			"date":       date,
			"calls":      day.Calls,
			"success":    day.Success,
			"failure":    day.Failure,
			"quota_used": day.QuotaUsed,
		})
	}
	return map[string]any{
		"call_count":    acc.Calls,
		"success_count": acc.Success,
		"failure_count": acc.Failure,
		"quota_used":    acc.QuotaUsed,
		"usage_curve":   curve,
	}
}

func logDay(item map[string]any) string {
	day := util.Clean(item["time"])
	if len(day) < 10 {
		return ""
	}
	return day[:10]
}

func logQuotaUsed(detail map[string]any, status string) int {
	if status != "success" {
		return 0
	}
	if urls := util.AsStringSlice(detail["urls"]); len(urls) > 0 {
		return len(urls)
	}
	endpoint := util.Clean(detail["endpoint"])
	switch endpoint {
	case "/v1/images/generations", "/v1/images/edits", "/api/image-tasks/generations", "/api/image-tasks/edits":
		return 1
	default:
		return 0
	}
}

type Logger struct {
	levels func() []string
}

func NewLogger(levels func() []string) *Logger {
	return &Logger{levels: levels}
}

func (l *Logger) enabled(level string) bool {
	levels := l.levels()
	if len(levels) == 0 {
		return level == "info" || level == "warning" || level == "error"
	}
	for _, item := range levels {
		if item == level {
			return true
		}
	}
	return false
}

func (l *Logger) Debug(v any)   { l.print("debug", v) }
func (l *Logger) Info(v any)    { l.print("info", v) }
func (l *Logger) Warning(v any) { l.print("warning", v) }
func (l *Logger) Error(v any)   { l.print("error", v) }

func (l *Logger) print(level string, v any) {
	if !l.enabled(level) {
		return
	}
	data, _ := json.Marshal(sanitize(v))
	os.Stderr.WriteString("[" + strings.ToUpper(level) + "] " + string(data) + "\n")
}

func sanitize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, value := range x {
			lower := strings.ToLower(key)
			if s, ok := value.(string); ok && (strings.Contains(lower, "token") || lower == "dx") {
				out[key] = maskString(s, 10)
			} else if s, ok := value.(string); ok && (strings.Contains(lower, "base64") || lower == "b64_json") {
				out[key] = maskBase64(s)
			} else {
				out[key] = sanitize(value)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = sanitize(item)
		}
		return out
	case []map[string]any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = sanitize(item)
		}
		return out
	case string:
		if strings.HasPrefix(strings.TrimSpace(x), "data:") && strings.Contains(x, ";base64,") {
			return maskBase64(x)
		}
		return x
	default:
		return v
	}
}

func maskString(value string, keep int) string {
	if len(value) <= keep {
		return value
	}
	return value[:keep] + "..."
}

func maskBase64(value string) string {
	if idx := strings.Index(value, ","); strings.HasPrefix(value, "data:") && idx >= 0 {
		return value[:idx+1] + maskString(value[idx+1:], 24)
	}
	return maskString(value, 24)
}

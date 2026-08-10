package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type PullSession struct {
	ID           string `json:"id"`
	ClientIP     string `json:"client_ip"`
	ImageName    string `json:"image_name"`
	Registry     string `json:"registry"`
	Tag          string `json:"tag"`
	Category     string `json:"category"`
	StartedAt    string `json:"started_at"`
	LastSeenAt   string `json:"last_seen_at"`
	CompletedAt  string `json:"completed_at,omitempty"`
	Status       string `json:"status"`
	BytesTotal   int64  `json:"bytes_total"`
	LayerCount   int    `json:"layer_count"`
	RequestCount int    `json:"request_count"`
	UserID       int64  `json:"user_id,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
}

type PullEvent struct {
	ID            int64  `json:"id"`
	PullSessionID string `json:"pull_session_id"`
	EventType     string `json:"event_type"`
	Reference     string `json:"reference,omitempty"`
	Bytes         int64  `json:"bytes"`
	StatusCode    int    `json:"status_code"`
	CreatedAt     string `json:"created_at"`
}

type DashboardStats struct {
	TotalPulls    int64            `json:"total_pulls"`
	TotalBytes    int64            `json:"total_bytes"`
	UniqueIPs     int64            `json:"unique_ips"`
	TodayPulls    int64            `json:"today_pulls"`
	TodayBytes    int64            `json:"today_bytes"`
	TopImages     []ImageStat      `json:"top_images"`
	TopIPs        []IPStat         `json:"top_ips"`
	CategoryStats []CategoryStat   `json:"category_stats"`
	DailyTrend    []DailyTrendItem `json:"daily_trend"`
	RecentPulls   []PullSession    `json:"recent_pulls"`
}

type ImageStat struct {
	ImageName  string `json:"image_name"`
	Registry   string `json:"registry"`
	Category   string `json:"category"`
	PullCount  int64  `json:"pull_count"`
	BytesTotal int64  `json:"bytes_total"`
	UniqueIPs  int64  `json:"unique_ips"`
}

type IPStat struct {
	ClientIP   string `json:"client_ip"`
	PullCount  int64  `json:"pull_count"`
	BytesTotal int64  `json:"bytes_total"`
	LastSeen   string `json:"last_seen"`
}

type CategoryStat struct {
	Category  string `json:"category"`
	PullCount int64  `json:"pull_count"`
	Bytes     int64  `json:"bytes_total"`
}

type DailyTrendItem struct {
	Day       string `json:"day"`
	PullCount int64  `json:"pull_count"`
	Bytes     int64  `json:"bytes_total"`
}

type PullListFilter struct {
	IP          string
	Image       string
	Category    string
	Registry    string
	Status      string
	From        string
	To          string
	UserID      int64
	Page        int
	PageSize    int
	CountedOnly bool // only sessions with at least one layer blob
}

func ImageCategory(imageName string) string {
	if imageName == "" {
		return "unknown"
	}
	if strings.HasPrefix(imageName, "library/") {
		return "library"
	}
	parts := strings.SplitN(imageName, "/", 2)
	if len(parts) == 1 {
		return "library"
	}
	return "user"
}

func FindActivePullSession(ip, imageName, registry string, userID int64) (*PullSession, error) {
	cfg := LoadPullSession()
	window := time.Duration(cfg.WindowMinutes) * time.Minute
	since := time.Now().UTC().Add(-window).Format(time.RFC3339Nano)
	var s PullSession
	var uid sql.NullInt64
	var tok sql.NullString
	err := DB.QueryRow(
		`SELECT id, client_ip, image_name, registry, tag, category, started_at, last_seen_at,
		        COALESCE(completed_at,''), status, bytes_total, layer_count, request_count,
		        user_id, access_token
		 FROM pull_sessions
		 WHERE client_ip = ? AND image_name = ? AND registry = ?
		   AND COALESCE(user_id,0) = ?
		   AND status = 'active' AND last_seen_at >= ?
		 ORDER BY last_seen_at DESC LIMIT 1`,
		ip, imageName, registry, userID, since,
	).Scan(
		&s.ID, &s.ClientIP, &s.ImageName, &s.Registry, &s.Tag, &s.Category,
		&s.StartedAt, &s.LastSeenAt, &s.CompletedAt, &s.Status,
		&s.BytesTotal, &s.LayerCount, &s.RequestCount, &uid, &tok,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if uid.Valid {
		s.UserID = uid.Int64
	}
	if tok.Valid {
		s.AccessToken = tok.String
	}
	return &s, nil
}

func isManifestEvent(eventType string) bool {
	return eventType == "manifests" || eventType == "manifest" || eventType == "manifest_head"
}

func isBlobEvent(eventType string) bool {
	return eventType == "blobs" || eventType == "blob"
}

// FindOrCreatePullSession implements pull cycle:
//  1. First manifest → open uncounted session (tracking only)
//  2. Blobs attach to that session; first successful blob → count +1, cycle "closed" for counting
//  3. More blobs of same pull still attach while session active
//  4. Next manifest after already counted (layer_count>0) → complete old, open NEW cycle
//  5. Manifest-only with no blob is deleted after ManifestProbeSeconds (see CleanupManifestProbes)
func FindOrCreatePullSession(ip, imageName, registry, tag, eventType string, userID int64, accessToken string) (*PullSession, bool, error) {
	if tag == "" {
		tag = "latest"
	}
	if registry == "" {
		registry = "docker.io"
	}
	category := ImageCategory(imageName)

	s, err := FindActivePullSession(ip, imageName, registry, userID)
	if err != nil {
		return nil, false, err
	}

	// Already counted this cycle → any new manifest starts a fresh cycle (second docker pull).
	// Digest manifests after first blob still belong to same pull (multi-arch follow-up is rare post-blob;
	// if tag/digest manifest after layers, treat as new pull intent).
	if s != nil && s.LayerCount > 0 && isManifestEvent(eventType) {
		_ = CompletePullSession(s.ID, "completed")
		s = nil
	}

	// Blob with no open uncounted/active session: open one (resume / race)
	if s != nil {
		now := Now()
		if tag != "" && tag != "digest" && !strings.HasPrefix(tag, "sha256:") &&
			(s.Tag == "" || s.Tag == "digest" || s.Tag == "latest" || s.Tag != tag) {
			_, _ = DB.Exec(
				`UPDATE pull_sessions SET last_seen_at = ?, request_count = request_count + 1, tag = ? WHERE id = ?`,
				now, tag, s.ID,
			)
			s.Tag = tag
		} else {
			_, _ = DB.Exec(
				`UPDATE pull_sessions SET last_seen_at = ?, request_count = request_count + 1 WHERE id = ?`,
				now, s.ID,
			)
		}
		s.LastSeenAt = now
		s.RequestCount++
		return s, false, nil
	}

	now := Now()
	id := uuid.NewString()
	var uid any
	if userID > 0 {
		uid = userID
	}
	var tok any
	if accessToken != "" {
		tok = accessToken
	}
	_, err = DB.Exec(
		`INSERT INTO pull_sessions
		 (id, client_ip, image_name, registry, tag, category, started_at, last_seen_at, status, bytes_total, layer_count, request_count, user_id, access_token)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active', 0, 0, 1, ?, ?)`,
		id, ip, imageName, registry, tag, category, now, now, uid, tok,
	)
	if err != nil {
		return nil, false, err
	}

	return &PullSession{
		ID:           id,
		ClientIP:     ip,
		ImageName:    imageName,
		Registry:     registry,
		Tag:          tag,
		Category:     category,
		StartedAt:    now,
		LastSeenAt:   now,
		Status:       "active",
		RequestCount: 1,
		UserID:       userID,
		AccessToken:  accessToken,
	}, true, nil
}

// countedPullSQL filters sessions that actually downloaded at least one layer blob.
const countedPullSQL = `layer_count > 0`

func CountPullsByUserInPeriod(userID int64, periodHours float64) (int, error) {
	since := time.Now().UTC().Add(-time.Duration(periodHours * float64(time.Hour))).Format(time.RFC3339Nano)
	var n int
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM pull_sessions WHERE user_id = ? AND started_at >= ?`,
		userID, since,
	).Scan(&n)
	return n, err
}

// LocalDayStart returns today's 00:00:00 in local timezone as RFC3339Nano UTC string bound.
func LocalDayStart() time.Time {
	now := time.Now()
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
}

func CountPullsByUserToday(userID int64) (int, error) {
	since := LocalDayStart().UTC().Format(time.RFC3339Nano)
	var n int
	// Only sessions that actually pulled at least one layer count as a real pull.
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM pull_sessions WHERE user_id = ? AND started_at >= ? AND `+countedPullSQL,
		userID, since,
	).Scan(&n)
	return n, err
}

type UserQuota struct {
	DailyLimit    int    `json:"daily_limit"`
	UsedToday     int    `json:"used_today"`
	Remaining     int    `json:"remaining"`
	ResetsAt      string `json:"resets_at"`
	ResetsAtHuman string `json:"resets_at_human"`
}

func GetUserQuota(userID int64) (*UserQuota, error) {
	u, err := GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	used, err := CountPullsByUserToday(userID)
	if err != nil {
		return nil, err
	}
	limit := u.DailyPullLimit
	if limit < 0 {
		limit = 0
	}
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	// next local midnight
	next := LocalDayStart().Add(24 * time.Hour)
	return &UserQuota{
		DailyLimit:    limit,
		UsedToday:     used,
		Remaining:     remaining,
		ResetsAt:      next.Format(time.RFC3339),
		ResetsAtHuman: next.Local().Format("2006-01-02 15:04"),
	}, nil
}

func GetUserDashboardStats(userID int64, days int) (*DashboardStats, error) {
	if days <= 0 {
		days = 14
	}
	stats := &DashboardStats{}
	_ = DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(bytes_total),0) FROM pull_sessions WHERE user_id = ? AND `+countedPullSQL, userID).
		Scan(&stats.TotalPulls, &stats.TotalBytes)
	_ = DB.QueryRow(`SELECT COUNT(DISTINCT client_ip) FROM pull_sessions WHERE user_id = ? AND `+countedPullSQL, userID).Scan(&stats.UniqueIPs)
	sinceToday := LocalDayStart().UTC().Format(time.RFC3339Nano)
	_ = DB.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(bytes_total),0) FROM pull_sessions WHERE user_id = ? AND started_at >= ? AND `+countedPullSQL,
		userID, sinceToday,
	).Scan(&stats.TodayPulls, &stats.TodayBytes)

	rows, err := DB.Query(
		`SELECT image_name, registry, category, COUNT(*), COALESCE(SUM(bytes_total),0), COUNT(DISTINCT client_ip)
		 FROM pull_sessions WHERE user_id = ? AND `+countedPullSQL+`
		 GROUP BY image_name, registry, category
		 ORDER BY COUNT(*) DESC LIMIT 10`, userID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var it ImageStat
			if err := rows.Scan(&it.ImageName, &it.Registry, &it.Category, &it.PullCount, &it.BytesTotal, &it.UniqueIPs); err == nil {
				stats.TopImages = append(stats.TopImages, it)
			}
		}
	}
	rows2, err := DB.Query(
		`SELECT client_ip, COUNT(*), COALESCE(SUM(bytes_total),0), MAX(last_seen_at)
		 FROM pull_sessions WHERE user_id = ? AND `+countedPullSQL+`
		 GROUP BY client_ip ORDER BY COUNT(*) DESC LIMIT 10`, userID,
	)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var it IPStat
			if err := rows2.Scan(&it.ClientIP, &it.PullCount, &it.BytesTotal, &it.LastSeen); err == nil {
				stats.TopIPs = append(stats.TopIPs, it)
			}
		}
	}
	stats.DailyTrend = fillDailyTrend(days, userID)
	recent, _, _ := ListPullSessions(PullListFilter{UserID: userID, Page: 1, PageSize: 30, CountedOnly: true})
	stats.RecentPulls = recent
	if stats.TopImages == nil {
		stats.TopImages = []ImageStat{}
	}
	if stats.TopIPs == nil {
		stats.TopIPs = []IPStat{}
	}
	if stats.CategoryStats == nil {
		stats.CategoryStats = []CategoryStat{}
	}
	if stats.RecentPulls == nil {
		stats.RecentPulls = []PullSession{}
	}
	return stats, nil
}

// fillDailyTrend returns a continuous series of `days` local calendar days (zeros filled).
func fillDailyTrend(days int, userID int64) []DailyTrendItem {
	if days <= 0 {
		days = 14
	}
	// use local day keys to match LocalDayStart / quota semantics
	type key struct {
		day   string
		pulls int64
		bytes int64
	}
	byDay := map[string]key{}

	// query last days+1 in UTC-ish filter, then group by local date in Go for correctness
	since := LocalDayStart().AddDate(0, 0, -days+1).UTC().Format(time.RFC3339Nano)
	var q string
	var args []any
	if userID > 0 {
		q = `SELECT started_at, bytes_total FROM pull_sessions WHERE user_id = ? AND started_at >= ? AND ` + countedPullSQL
		args = []any{userID, since}
	} else {
		q = `SELECT started_at, bytes_total FROM pull_sessions WHERE started_at >= ? AND ` + countedPullSQL
		args = []any{since}
	}
	rows, err := DB.Query(q, args...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var started string
			var bytes int64
			if err := rows.Scan(&started, &bytes); err != nil {
				continue
			}
			t, err := ParseTime(started)
			if err != nil {
				// fallback: first 10 chars
				if len(started) >= 10 {
					d := started[:10]
					k := byDay[d]
					k.day = d
					k.pulls++
					k.bytes += bytes
					byDay[d] = k
				}
				continue
			}
			d := t.In(time.Local).Format("2006-01-02")
			k := byDay[d]
			k.day = d
			k.pulls++
			k.bytes += bytes
			byDay[d] = k
		}
	}

	out := make([]DailyTrendItem, 0, days)
	start := LocalDayStart().AddDate(0, 0, -days+1)
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i).Format("2006-01-02")
		if k, ok := byDay[d]; ok {
			out = append(out, DailyTrendItem{Day: d, PullCount: k.pulls, Bytes: k.bytes})
		} else {
			out = append(out, DailyTrendItem{Day: d, PullCount: 0, Bytes: 0})
		}
	}
	return out
}

func RecordPullEvent(sessionID, eventType, reference string, bytes int64, statusCode int) error {
	now := Now()
	_, err := DB.Exec(
		`INSERT INTO pull_events (pull_session_id, event_type, reference, bytes, status_code, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, eventType, reference, bytes, statusCode, now,
	)
	if err != nil {
		return err
	}

	isBlob := isBlobEvent(eventType)
	// Any successful blob response counts as a layer for this pull cycle
	countAsLayer := isBlob && statusCode >= 200 && statusCode < 400

	if bytes > 0 || countAsLayer {
		// Atomic: only first layer bumps daily pull count
		if countAsLayer {
			res, err2 := DB.Exec(
				`UPDATE pull_sessions SET
				   bytes_total = bytes_total + ?,
				   layer_count = layer_count + 1,
				   last_seen_at = ?
				 WHERE id = ? AND layer_count = 0`,
				bytes, now, sessionID,
			)
			if err2 != nil {
				return err2
			}
			if n, _ := res.RowsAffected(); n > 0 {
				// first blob of this cycle → count once
				_ = bumpDailyPull(now[:10])
				if bytes > 0 {
					_ = bumpDailyBytes(now[:10], bytes)
				}
				return nil
			}
			// already counted: just add traffic / layer stats
			_, err = DB.Exec(
				`UPDATE pull_sessions SET
				   bytes_total = bytes_total + ?,
				   layer_count = layer_count + 1,
				   last_seen_at = ?
				 WHERE id = ?`,
				bytes, now, sessionID,
			)
			if err != nil {
				return err
			}
			if bytes > 0 {
				_ = bumpDailyBytes(now[:10], bytes)
			}
			return nil
		}
		// non-blob with bytes (e.g. manifest body)
		_, err = DB.Exec(
			`UPDATE pull_sessions SET bytes_total = bytes_total + ?, last_seen_at = ? WHERE id = ?`,
			bytes, now, sessionID,
		)
		if err != nil {
			return err
		}
		if bytes > 0 {
			_ = bumpDailyBytes(now[:10], bytes)
		}
	} else {
		_, _ = DB.Exec(`UPDATE pull_sessions SET last_seen_at = ? WHERE id = ?`, now, sessionID)
	}
	return nil
}

func CompletePullSession(sessionID, status string) error {
	if status == "" {
		status = "completed"
	}
	now := Now()
	_, err := DB.Exec(
		`UPDATE pull_sessions SET status = ?, completed_at = ?, last_seen_at = ? WHERE id = ? AND status = 'active'`,
		status, now, now, sessionID,
	)
	return err
}

func ExpireIdlePullSessions() error {
	cfg := LoadPullSession()
	idle := time.Duration(cfg.IdleMinutes) * time.Minute
	cutoff := time.Now().UTC().Add(-idle).Format(time.RFC3339Nano)
	now := Now()
	// Only expire sessions that already counted (have layers); probes cleaned separately
	_, err := DB.Exec(
		`UPDATE pull_sessions SET status = 'completed', completed_at = ?
		 WHERE status = 'active' AND layer_count > 0 AND last_seen_at < ?`,
		now, cutoff,
	)
	return err
}

// CleanupManifestProbes removes sessions that only saw manifest (no blob) after probe timeout.
func CleanupManifestProbes() error {
	cfg := LoadPullSession()
	sec := cfg.ManifestProbeSeconds
	if sec <= 0 {
		sec = 60
	}
	cutoff := time.Now().UTC().Add(-time.Duration(sec) * time.Second).Format(time.RFC3339Nano)
	// delete events first if any (usually none for pure probe)
	_, _ = DB.Exec(
		`DELETE FROM pull_events WHERE pull_session_id IN (
			SELECT id FROM pull_sessions WHERE status = 'active' AND layer_count = 0 AND last_seen_at < ?
		)`,
		cutoff,
	)
	_, err := DB.Exec(
		`DELETE FROM pull_sessions WHERE status = 'active' AND layer_count = 0 AND last_seen_at < ?`,
		cutoff,
	)
	return err
}

func CleanupOldPullData() error {
	cfg := LoadPullSession()
	days := cfg.RetentionDays
	if days <= 0 {
		days = 90
	}
	cutoffTime := LocalDayStart().AddDate(0, 0, -days)
	cutoff := cutoffTime.UTC().Format(time.RFC3339Nano)
	cutoffDay := cutoffTime.Format("2006-01-02")

	_, _ = DB.Exec(
		`DELETE FROM pull_events WHERE pull_session_id IN (
			SELECT id FROM pull_sessions WHERE started_at < ?
		)`,
		cutoff,
	)
	if _, err := DB.Exec(`DELETE FROM pull_sessions WHERE started_at < ?`, cutoff); err != nil {
		return err
	}
	_, err := DB.Exec(`DELETE FROM daily_stats WHERE day < ?`, cutoffDay)
	return err
}

func bumpDailyPull(day string) error {
	_, err := DB.Exec(
		`INSERT INTO daily_stats (day, pull_count, bytes_total, unique_ips) VALUES (?, 1, 0, 0)
		 ON CONFLICT(day) DO UPDATE SET pull_count = pull_count + 1`,
		day,
	)
	return err
}

func bumpDailyBytes(day string, bytes int64) error {
	_, err := DB.Exec(
		`INSERT INTO daily_stats (day, pull_count, bytes_total, unique_ips) VALUES (?, 0, ?, 0)
		 ON CONFLICT(day) DO UPDATE SET bytes_total = bytes_total + ?`,
		day, bytes, bytes,
	)
	return err
}

func CountPullsByIPInPeriod(ip string, periodHours float64) (int, error) {
	since := time.Now().UTC().Add(-time.Duration(periodHours * float64(time.Hour))).Format(time.RFC3339Nano)
	var n int
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM pull_sessions WHERE client_ip = ? AND started_at >= ? AND `+countedPullSQL,
		ip, since,
	).Scan(&n)
	return n, err
}

func GetDashboardStats(days int) (*DashboardStats, error) {
	if days <= 0 {
		days = 14
	}
	stats := &DashboardStats{}

	_ = DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(bytes_total),0) FROM pull_sessions WHERE `+countedPullSQL).Scan(&stats.TotalPulls, &stats.TotalBytes)
	_ = DB.QueryRow(`SELECT COUNT(DISTINCT client_ip) FROM pull_sessions WHERE ` + countedPullSQL).Scan(&stats.UniqueIPs)

	sinceToday := LocalDayStart().UTC().Format(time.RFC3339Nano)
	_ = DB.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(bytes_total),0) FROM pull_sessions WHERE started_at >= ? AND `+countedPullSQL,
		sinceToday,
	).Scan(&stats.TodayPulls, &stats.TodayBytes)

	rows, err := DB.Query(
		`SELECT image_name, registry, category, COUNT(*), COALESCE(SUM(bytes_total),0), COUNT(DISTINCT client_ip)
		 FROM pull_sessions WHERE ` + countedPullSQL + `
		 GROUP BY image_name, registry, category
		 ORDER BY COUNT(*) DESC LIMIT 10`,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var it ImageStat
			if err := rows.Scan(&it.ImageName, &it.Registry, &it.Category, &it.PullCount, &it.BytesTotal, &it.UniqueIPs); err == nil {
				stats.TopImages = append(stats.TopImages, it)
			}
		}
	}

	rows2, err := DB.Query(
		`SELECT client_ip, COUNT(*), COALESCE(SUM(bytes_total),0), MAX(last_seen_at)
		 FROM pull_sessions WHERE ` + countedPullSQL + `
		 GROUP BY client_ip ORDER BY COUNT(*) DESC LIMIT 10`,
	)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var it IPStat
			if err := rows2.Scan(&it.ClientIP, &it.PullCount, &it.BytesTotal, &it.LastSeen); err == nil {
				stats.TopIPs = append(stats.TopIPs, it)
			}
		}
	}

	rows3, err := DB.Query(
		`SELECT category, COUNT(*), COALESCE(SUM(bytes_total),0)
		 FROM pull_sessions WHERE ` + countedPullSQL + `
		 GROUP BY category ORDER BY COUNT(*) DESC`,
	)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var it CategoryStat
			if err := rows3.Scan(&it.Category, &it.PullCount, &it.Bytes); err == nil {
				stats.CategoryStats = append(stats.CategoryStats, it)
			}
		}
	}

	stats.DailyTrend = fillDailyTrend(days, 0)

	recent, _, err := ListPullSessions(PullListFilter{Page: 1, PageSize: 30, CountedOnly: true})
	if err == nil {
		stats.RecentPulls = recent
	}
	if stats.TopImages == nil {
		stats.TopImages = []ImageStat{}
	}
	if stats.TopIPs == nil {
		stats.TopIPs = []IPStat{}
	}
	if stats.CategoryStats == nil {
		stats.CategoryStats = []CategoryStat{}
	}
	if stats.RecentPulls == nil {
		stats.RecentPulls = []PullSession{}
	}
	return stats, nil
}

func ListPullSessions(f PullListFilter) ([]PullSession, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}

	where := []string{"1=1"}
	args := []any{}
	if f.IP != "" {
		where = append(where, "client_ip LIKE ?")
		args = append(args, "%"+f.IP+"%")
	}
	if f.Image != "" {
		where = append(where, "image_name LIKE ?")
		args = append(args, "%"+f.Image+"%")
	}
	if f.Category != "" {
		where = append(where, "category = ?")
		args = append(args, f.Category)
	}
	if f.Registry != "" {
		where = append(where, "registry = ?")
		args = append(args, f.Registry)
	}
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.From != "" {
		where = append(where, "started_at >= ?")
		args = append(args, f.From)
	}
	if f.To != "" {
		where = append(where, "started_at <= ?")
		args = append(args, f.To)
	}
	if f.UserID > 0 {
		where = append(where, "user_id = ?")
		args = append(args, f.UserID)
	}
	if f.CountedOnly {
		where = append(where, countedPullSQL)
	}
	clause := strings.Join(where, " AND ")

	var total int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM pull_sessions WHERE `+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (f.Page - 1) * f.PageSize
	query := fmt.Sprintf(
		`SELECT id, client_ip, image_name, registry, tag, category, started_at, last_seen_at,
		        COALESCE(completed_at,''), status, bytes_total, layer_count, request_count,
		        COALESCE(user_id,0), COALESCE(access_token,'')
		 FROM pull_sessions WHERE %s
		 ORDER BY started_at DESC LIMIT ? OFFSET ?`, clause,
	)
	qArgs := append(append([]any{}, args...), f.PageSize, offset)
	rows, err := DB.Query(query, qArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []PullSession
	for rows.Next() {
		var s PullSession
		if err := rows.Scan(
			&s.ID, &s.ClientIP, &s.ImageName, &s.Registry, &s.Tag, &s.Category,
			&s.StartedAt, &s.LastSeenAt, &s.CompletedAt, &s.Status,
			&s.BytesTotal, &s.LayerCount, &s.RequestCount, &s.UserID, &s.AccessToken,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, s)
	}
	if list == nil {
		list = []PullSession{}
	}
	return list, total, rows.Err()
}

func GetPullSession(id string) (*PullSession, []PullEvent, error) {
	s := &PullSession{}
	err := DB.QueryRow(
		`SELECT id, client_ip, image_name, registry, tag, category, started_at, last_seen_at,
		        COALESCE(completed_at,''), status, bytes_total, layer_count, request_count,
		        COALESCE(user_id,0), COALESCE(access_token,'')
		 FROM pull_sessions WHERE id = ?`, id,
	).Scan(
		&s.ID, &s.ClientIP, &s.ImageName, &s.Registry, &s.Tag, &s.Category,
		&s.StartedAt, &s.LastSeenAt, &s.CompletedAt, &s.Status,
		&s.BytesTotal, &s.LayerCount, &s.RequestCount, &s.UserID, &s.AccessToken,
	)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("not found")
	}
	if err != nil {
		return nil, nil, err
	}

	rows, err := DB.Query(
		`SELECT id, pull_session_id, event_type, COALESCE(reference,''), bytes, status_code, created_at
		 FROM pull_events WHERE pull_session_id = ? ORDER BY id ASC`, id,
	)
	if err != nil {
		return s, []PullEvent{}, nil
	}
	defer rows.Close()
	var events []PullEvent
	for rows.Next() {
		var e PullEvent
		if err := rows.Scan(&e.ID, &e.PullSessionID, &e.EventType, &e.Reference, &e.Bytes, &e.StatusCode, &e.CreatedAt); err != nil {
			return nil, nil, err
		}
		events = append(events, e)
	}
	if events == nil {
		events = []PullEvent{}
	}
	return s, events, nil
}

func ListImageStats(image, category, registry string, page, pageSize int) ([]ImageStat, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	where := []string{"1=1"}
	args := []any{}
	if image != "" {
		where = append(where, "image_name LIKE ?")
		args = append(args, "%"+image+"%")
	}
	if category != "" {
		where = append(where, "category = ?")
		args = append(args, category)
	}
	if registry != "" {
		where = append(where, "registry = ?")
		args = append(args, registry)
	}
	clause := strings.Join(where, " AND ")

	var total int
	countQ := `SELECT COUNT(*) FROM (
		SELECT 1 FROM pull_sessions WHERE ` + clause + ` GROUP BY image_name, registry, category
	)`
	if err := DB.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	q := fmt.Sprintf(
		`SELECT image_name, registry, category, COUNT(*), COALESCE(SUM(bytes_total),0), COUNT(DISTINCT client_ip)
		 FROM pull_sessions WHERE %s
		 GROUP BY image_name, registry, category
		 ORDER BY COUNT(*) DESC LIMIT ? OFFSET ?`, clause,
	)
	qArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := DB.Query(q, qArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var list []ImageStat
	for rows.Next() {
		var it ImageStat
		if err := rows.Scan(&it.ImageName, &it.Registry, &it.Category, &it.PullCount, &it.BytesTotal, &it.UniqueIPs); err != nil {
			return nil, 0, err
		}
		list = append(list, it)
	}
	if list == nil {
		list = []ImageStat{}
	}
	return list, total, rows.Err()
}

func ListIPStats(ip string, page, pageSize int) ([]IPStat, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	where := "1=1"
	args := []any{}
	if ip != "" {
		where = "client_ip LIKE ?"
		args = append(args, "%"+ip+"%")
	}
	var total int
	if err := DB.QueryRow(
		`SELECT COUNT(*) FROM (SELECT 1 FROM pull_sessions WHERE `+where+` GROUP BY client_ip)`,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	qArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := DB.Query(
		`SELECT client_ip, COUNT(*), COALESCE(SUM(bytes_total),0), MAX(last_seen_at)
		 FROM pull_sessions WHERE `+where+`
		 GROUP BY client_ip ORDER BY COUNT(*) DESC LIMIT ? OFFSET ?`,
		qArgs...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var list []IPStat
	for rows.Next() {
		var it IPStat
		if err := rows.Scan(&it.ClientIP, &it.PullCount, &it.BytesTotal, &it.LastSeen); err != nil {
			return nil, 0, err
		}
		list = append(list, it)
	}
	if list == nil {
		list = []IPStat{}
	}
	return list, total, rows.Err()
}

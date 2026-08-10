package db

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

// SeedDemoPulls inserts sample pull sessions for local UI preview when:
//   HUBPROXY_SEED_DEMO=1  and  pull table has fewer than 5 counted sessions.
func SeedDemoPulls() error {
	if os.Getenv("HUBPROXY_SEED_DEMO") != "1" && os.Getenv("SEED_DEMO") != "1" {
		return nil
	}
	var n int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM pull_sessions WHERE ` + countedPullSQL).Scan(&n); err != nil {
		return err
	}
	if n >= 5 {
		return nil
	}

	// attach to first user if any
	var userID any
	var tok any
	var uid int64
	if err := DB.QueryRow(`SELECT id FROM users ORDER BY id ASC LIMIT 1`).Scan(&uid); err == nil {
		userID = uid
		if at, err := EnsureUserAccessToken(uid); err == nil {
			tok = at.Token
		}
	}

	type sample struct {
		ip, image, registry, tag, cat string
		layers                        int
		bytes                         int64
		daysAgo                       int
		hour                          int
	}
	samples := []sample{
		{"203.0.113.10", "library/nginx", "docker.io", "latest", "library", 6, 28_000_000, 0, 10},
		{"203.0.113.10", "library/redis", "docker.io", "7", "library", 4, 12_000_000, 0, 14},
		{"198.51.100.22", "library/alpine", "docker.io", "3.19", "library", 1, 3_500_000, 1, 9},
		{"198.51.100.22", "library/postgres", "docker.io", "16", "library", 8, 95_000_000, 1, 18},
		{"203.0.113.45", "bitnami/mysql", "docker.io", "8.0", "user", 10, 120_000_000, 2, 11},
		{"203.0.113.45", "owner/app", "ghcr.io", "v1.2.0", "user", 5, 40_000_000, 3, 16},
		{"192.0.2.88", "library/node", "docker.io", "20", "library", 7, 55_000_000, 4, 8},
		{"192.0.2.88", "library/python", "docker.io", "3.12", "library", 6, 48_000_000, 5, 20},
		{"203.0.113.10", "pause", "registry.k8s.io", "3.9", "library", 1, 800_000, 6, 12},
		{"198.51.100.22", "library/busybox", "docker.io", "latest", "library", 1, 1_200_000, 7, 15},
		{"203.0.113.99", "group/project", "registry.gitlab.com", "main", "user", 4, 22_000_000, 8, 10},
		{"203.0.113.10", "library/nginx", "docker.io", "1.25", "library", 6, 30_000_000, 9, 19},
		{"192.0.2.15", "library/memcached", "docker.io", "latest", "library", 2, 5_000_000, 10, 7},
		{"203.0.113.45", "library/mongo", "docker.io", "7", "library", 9, 150_000_000, 11, 13},
		{"198.51.100.22", "library/httpd", "docker.io", "2.4", "library", 3, 18_000_000, 12, 17},
		{"203.0.113.10", "library/traefik", "docker.io", "v3.0", "library", 5, 35_000_000, 13, 11},
	}

	now := time.Now()
	for _, s := range samples {
		t := time.Date(now.Year(), now.Month(), now.Day(), s.hour, 12, 0, 0, now.Location()).
			AddDate(0, 0, -s.daysAgo)
		started := t.UTC().Format(time.RFC3339Nano)
		id := uuid.NewString()
		_, err := DB.Exec(
			`INSERT INTO pull_sessions
			 (id, client_ip, image_name, registry, tag, category, started_at, last_seen_at, completed_at, status,
			  bytes_total, layer_count, request_count, user_id, access_token)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'completed', ?, ?, ?, ?, ?)`,
			id, s.ip, s.image, s.registry, s.tag, s.cat, started, started, started,
			s.bytes, s.layers, s.layers+2, userID, tok,
		)
		if err != nil {
			return fmt.Errorf("seed pull: %w", err)
		}
		_ = bumpDailyPull(started[:10])
		_ = bumpDailyBytes(started[:10], s.bytes)
	}
	fmt.Printf("已写入 %d 条演示拉取数据（HUBPROXY_SEED_DEMO=1）\n", len(samples))
	return nil
}

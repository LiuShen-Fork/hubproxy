package db

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

// demoUserSlots 表示一条演示记录挂在谁名下。
const (
	demoAnon    = -1 // user_id 为 NULL：显示为「匿名」
	demoDeleted = -2 // 指向一个不存在的用户：显示为「#<id>」
)

// SeedDemoPulls 在满足以下条件时写入演示拉取数据：
//
//	环境变量 HUBPROXY_SEED_DEMO=1（或 SEED_DEMO=1）且已计次会话少于 5 条。
//
// 数据形状是**刻意设计**的，为的是让管理后台的筛选与列都有东西可看：
//
//   - 多个用户，且**存在被两个用户共用的 IP**——IP 分析页的多用户标记靠它才会出现
//   - 一条匿名记录（user_id 为 NULL）与一条指向已删除用户的记录（显示 #<id>）
//   - 覆盖全部内置 Registry，外加一个**已从设置中移除**的（nvcr.io）——
//     后者的历史行仍在表里，来源下拉必须能筛到它
//   - library 与 user 两种类别都有
//   - 时间从今天一直铺到 70 天前，使「今天 / 近 7 天 / 近 30 天 / 全部」结果各不相同
//
// 所有 IP 都取自 RFC 5737 的文档专用网段（192.0.2.0/24、198.51.100.0/24、
// 203.0.113.0/24），因此演示数据可以整批删除而不误伤真实记录：
//
//	DELETE FROM pull_sessions
//	 WHERE client_ip LIKE '192.0.2.%' OR client_ip LIKE '198.51.100.%'
//	    OR client_ip LIKE '203.0.113.%';
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

	userIDs, err := demoUserIDs()
	if err != nil {
		return err
	}

	type sample struct {
		ip, image, registry, tag, cat string
		layers                        int
		bytes                         int64
		daysAgo, hour                 int
		slot                          int
	}
	samples := []sample{
		// 203.0.113.10 被两个不同用户使用——IP 分析页会标出这个信号
		{"203.0.113.10", "library/nginx", "docker.io", "latest", "library", 6, 28_000_000, 0, 10, 0},
		{"203.0.113.10", "library/redis", "docker.io", "7", "library", 4, 12_000_000, 0, 14, 1},
		// 198.51.100.22 同样被两人共用
		{"198.51.100.22", "library/alpine", "docker.io", "3.19", "library", 1, 3_500_000, 1, 9, 0},
		{"198.51.100.22", "library/postgres", "docker.io", "16", "library", 8, 95_000_000, 2, 18, 1},
		{"203.0.113.45", "bitnami/mysql", "docker.io", "8.0", "user", 10, 120_000_000, 3, 11, demoAnon},
		{"203.0.113.45", "owner/app", "ghcr.io", "v1.2.0", "user", 5, 40_000_000, 3, 16, demoDeleted},
		{"192.0.2.88", "org/model", "gcr.io", "1.0", "user", 4, 66_000_000, 3, 20, 0},
		{"203.0.113.77", "team/tool", "quay.io", "2.1", "user", 3, 21_000_000, 1, 13, 1},
		{"192.0.2.88", "library/node", "docker.io", "20", "library", 7, 55_000_000, 5, 8, 0},
		{"192.0.2.88", "library/python", "docker.io", "3.12", "library", 6, 48_000_000, 6, 20, 1},
		{"203.0.113.10", "pause", "registry.k8s.io", "3.9", "library", 1, 800_000, 8, 12, 0},
		{"198.51.100.22", "library/busybox", "docker.io", "latest", "library", 1, 1_200_000, 12, 15, demoAnon},
		{"203.0.113.99", "group/project", "registry.gitlab.com", "main", "user", 4, 22_000_000, 20, 10, 0},
		{"203.0.113.10", "library/nginx", "docker.io", "1.25", "library", 6, 30_000_000, 25, 19, 1},
		{"192.0.2.15", "library/memcached", "docker.io", "latest", "library", 2, 5_000_000, 28, 7, demoAnon},
		// nvcr.io 已从设置里移除，但历史行仍在——来源下拉必须能列出它
		{"203.0.113.77", "ai/model", "nvcr.io", "23.10", "user", 12, 180_000_000, 2, 16, 0},
		{"203.0.113.45", "library/mongo", "docker.io", "7", "library", 9, 150_000_000, 40, 13, 1},
		{"198.51.100.22", "library/httpd", "docker.io", "2.4", "library", 3, 18_000_000, 55, 17, 0},
		{"203.0.113.10", "library/traefik", "docker.io", "v3.0", "library", 5, 35_000_000, 70, 11, 1},
	}

	now := time.Now()
	for _, s := range samples {
		started := time.Date(now.Year(), now.Month(), now.Day(), s.hour, 12, 0, 0, now.Location()).
			AddDate(0, 0, -s.daysAgo).UTC().Format(time.RFC3339Nano)

		var userID any
		switch {
		case s.slot == demoAnon:
			userID = nil
		case s.slot == demoDeleted:
			userID = int64(999999) // 不存在，用于演示「用户已删除」的 #<id> 渲染
		case len(userIDs) > 0:
			userID = userIDs[s.slot%len(userIDs)]
		}

		if _, err := DB.Exec(
			`INSERT INTO pull_sessions
			 (id, client_ip, image_name, registry, tag, category, started_at, last_seen_at, completed_at, status,
			  bytes_total, layer_count, request_count, user_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'completed', ?, ?, ?, ?)`,
			uuid.NewString(), s.ip, s.image, s.registry, s.tag, s.cat,
			started, started, started, s.bytes, s.layers, s.layers+2, userID,
		); err != nil {
			return fmt.Errorf("seed pull: %w", err)
		}
		_ = bumpDailyPull(started[:10])
		_ = bumpDailyBytes(started[:10], s.bytes)
	}

	fmt.Printf("已写入 %d 条演示拉取数据（HUBPROXY_SEED_DEMO=1）\n", len(samples))
	fmt.Printf("  用户 %d 个，含匿名与已删除用户各一条；两处 IP 被多个用户共用\n", len(userIDs))
	return nil
}

// demoUserIDs 取现有用户的 id。演示数据不创建账号：库里有多少就挂多少个，
// 少于两个时多用户筛选与共用 IP 的标记就展示不出来，但也不该为此造账号。
func demoUserIDs() ([]int64, error) {
	rows, err := DB.Query(`SELECT id FROM users ORDER BY id ASC LIMIT 3`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

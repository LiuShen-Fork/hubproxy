package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"hubproxy/db"
)

// 来源下拉的接口即使一条数据都没有，也必须回 {"items":[]}。
// 回 "items": null 会让前端的 res.items.length / .map 直接抛 TypeError。
func TestAdminListRegistriesNeverReturnsNullItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := db.Init(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	AdminListRegistries(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200（响应 %s）", w.Code, w.Body.String())
	}
	var body struct {
		Items []string `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应 %q 失败：%v", w.Body.String(), err)
	}
	if body.Items == nil {
		t.Fatalf("items 必须是数组而不是 null，实际响应 %s", w.Body.String())
	}
}

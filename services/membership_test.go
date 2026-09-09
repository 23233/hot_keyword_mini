// Package services membership_test.go
package services

import (
	"hot_keyword/models"
	"testing"
	"time"
)

// TestCalculateMembershipGrant 验证同级顺延、升级折算与订单幂等。
func TestCalculateMembershipGrant(t *testing.T) {
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name          string
		currentLevel  int
		remainingDays int
		planLevel     int
		planDays      int
		wantDays      int
	}{
		{name: "level1剩余30天升级level2折算15天", currentLevel: 1, remainingDays: 30, planLevel: 2, planDays: 90, wantDays: 105},
		{name: "level2剩余90天升级level3折算60天", currentLevel: 2, remainingDays: 90, planLevel: 3, planDays: 365, wantDays: 425},
		{name: "同等级从原到期时间顺延", currentLevel: 1, remainingDays: 10, planLevel: 1, planDays: 30, wantDays: 40},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := &models.UserMembership{AppID: "wx_test", UserID: 1, Level: tt.currentLevel, ExpiresAt: now.AddDate(0, 0, tt.remainingDays), LastOrderID: 10}
			order := &models.PaymentOrder{ID: 11, AppID: "wx_test", UserID: 1}
			got, changed, err := calculateMembershipGrant(current, models.MembershipLevel{Level: tt.planLevel, DurationDays: tt.planDays}, order, now)
			if err != nil || !changed {
				t.Fatalf("会员权益计算失败: changed=%t err=%v", changed, err)
			}
			if got.Level != tt.planLevel || !got.ExpiresAt.Equal(now.AddDate(0, 0, tt.wantDays)) {
				t.Fatalf("权益结果不符合预期: level=%d expires=%s", got.Level, got.ExpiresAt)
			}
		})
	}

	current := &models.UserMembership{AppID: "wx_test", UserID: 1, Level: 2, ExpiresAt: now.AddDate(0, 0, 90), LastOrderID: 99}
	got, changed, err := calculateMembershipGrant(current, models.MembershipLevel{Level: 2, DurationDays: 90}, &models.PaymentOrder{ID: 99, AppID: "wx_test", UserID: 1}, now)
	if err != nil || changed || !got.ExpiresAt.Equal(current.ExpiresAt) {
		t.Fatalf("重复订单不应再次延长权益: changed=%t err=%v", changed, err)
	}
}

// TestArticleCanRead 验证文章最低会员等级采用大于等于规则。
func TestArticleCanRead(t *testing.T) {
	for _, tt := range []struct {
		required, level int
		want            bool
	}{{0, 0, true}, {1, 0, false}, {2, 2, true}, {2, 3, true}, {3, 2, false}} {
		if got := articleCanRead(tt.required, tt.level); got != tt.want {
			t.Fatalf("required=%d level=%d: got=%t want=%t", tt.required, tt.level, got, tt.want)
		}
	}
	paid := models.Article{IsPaid: true, RequiredLevel: 0}
	if articleCanReadForViewer(paid, 3, false) || !articleCanReadForViewer(paid, 0, true) {
		t.Fatal("单篇付费文章必须购买后才能阅读，购买后不受会员等级限制")
	}
}

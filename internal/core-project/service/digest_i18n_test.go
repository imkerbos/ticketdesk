package service

import (
	"strings"
	"testing"

	"github.com/kerbos/ticketdesk/pkg/i18n"
)

// TestDigestGroupLabelsFollowSiteLanguage 日报的分组标签跟随全局配置
//
// 群日报是一条消息发给整群，没有"收件人"这个概念，
// 语言只能是站点级的 —— 这条测试就是钉住这个约定。
func TestDigestGroupLabelsFollowSiteLanguage(t *testing.T) {
	orig := i18n.BackgroundLang()
	defer i18n.SetBackgroundLang(string(orig))

	i18n.SetBackgroundLang("zh-CN")
	if got := i18n.B("notify.digest_unassigned"); got != "未指派" {
		t.Errorf("中文站点下的未指派分组名 = %q", got)
	}
	if got := i18n.Bf("notify.digest_user_fallback", 42); got != "用户#42" {
		t.Errorf("中文站点下的用户兜底名 = %q", got)
	}

	i18n.SetBackgroundLang("en-US")
	if got := i18n.B("notify.digest_unassigned"); got != "Unassigned" {
		t.Errorf("英文站点下的未指派分组名 = %q", got)
	}
	if got := i18n.Bf("notify.digest_user_fallback", 42); got != "User #42" {
		t.Errorf("英文站点下的用户兜底名 = %q", got)
	}
	// 日报表头同样跟着切
	if got := i18n.Bf("notify.digest_header", "OPS", "Ops", 3); !strings.Contains(got, "open issues") {
		t.Errorf("英文站点下的日报表头 = %q", got)
	}
}

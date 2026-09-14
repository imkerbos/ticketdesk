package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/pkg/i18n"
	"github.com/kerbos/ticketdesk/pkg/logger"
	"github.com/kerbos/ticketdesk/pkg/safego"
)

// languageSyncInterval 副本间同步平台语言的间隔。
//
// 改语言是低频的管理动作，不值得为它引一套跨副本推送；
// 但默认部署就是 2 副本，只让处理保存的那个生效等于日报有一半几率发旧语言。
// 30 秒是"管理员点完保存、切到群里看效果"这个动作的自然容忍度。
const languageSyncInterval = 30 * time.Second

// ApplyLanguage 从数据库读取平台语言并应用；读不到或值不合法时保持现状
//
// 平台语言只有数据库这一个来源（种子值 zh-CN）。读失败时保持现状而不是回落默认，
// 是因为一次数据库抖动不该让整站语言悄悄变掉。
func ApplyLanguage(ctx context.Context, svc ConfigService) {
	lang, err := svc.GetConfigValue(ctx, KeyGeneralLanguage)
	if err != nil {
		logger.Warn("failed to read platform language, keeping current",
			zap.String("current", string(i18n.BackgroundLang())),
			zap.Error(err),
		)
		return
	}
	i18n.SetBackgroundLang(lang)
}

// StartLanguageSync 周期性把数据库里的平台语言同步到本副本
//
// 管理员保存时本副本会立刻生效（见 UpdateConfig），
// 这个循环负责让其余副本在一个间隔内收敛，不必等下次发版。
func StartLanguageSync(ctx context.Context, svc ConfigService) {
	ApplyLanguage(ctx, svc)

	safego.Go("config.languageSync", func() {
		ticker := time.NewTicker(languageSyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ApplyLanguage(ctx, svc)
			}
		}
	})
}

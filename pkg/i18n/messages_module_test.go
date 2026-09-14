package i18n

import (
	"strings"
	"sync"
	"testing"
)

// TestModuleKeysAreAscii 消息 key 必须是纯 ASCII 点分名。
//
// response.Error 用 Has(message) 判断"这串是不是 key"来决定翻不翻译。
// 一旦有人把中文塞进 key，那条判断就可能误伤真正的文案。
func TestModuleKeysAreAscii(t *testing.T) {
	for _, key := range Keys() {
		for _, r := range key {
			if r > 127 {
				t.Errorf("key 含非 ASCII 字符: %q", key)
				break
			}
		}
		if !strings.Contains(key, ".") {
			t.Errorf("key 应是点分域名式: %q", key)
		}
	}
}

// TestBackgroundLang 后台语言只接受支持的值，其余保持原样
func TestBackgroundLang(t *testing.T) {
	orig := BackgroundLang()
	defer SetBackgroundLang(string(orig))

	SetBackgroundLang("en-US")
	if BackgroundLang() != EnUS {
		t.Fatalf("SetBackgroundLang(en-US) = %v", BackgroundLang())
	}
	if got := B("notify.label_status"); got != "Status" {
		t.Errorf("B 未按后台语言取文案: %q", got)
	}

	// 无法识别的值不应把语言重置成默认，否则一个笔误会静默切回中文
	SetBackgroundLang("de-DE")
	if BackgroundLang() != EnUS {
		t.Errorf("无法识别的语言不该改变现有设置，实际 %v", BackgroundLang())
	}

	SetBackgroundLang("zh-CN")
	if got := B("notify.label_status"); got != "状态" {
		t.Errorf("切回中文失败: %q", got)
	}
}

// TestHasOnlyMatchesKeys Has 不能把普通文案误判成 key
func TestHasOnlyMatchesKeys(t *testing.T) {
	for _, s := range []string{"工单不存在", "Issue not found", "请求参数错误: xxx", ""} {
		if Has(s) {
			t.Errorf("Has(%q) 不应为 true", s)
		}
	}
	if !Has("issue.not_found") {
		t.Error("Has 应认得已登记的 key")
	}
}

// TestResolveLang 用户偏好语言的解析与回落
func TestResolveLang(t *testing.T) {
	orig := BackgroundLang()
	defer SetBackgroundLang(string(orig))
	SetBackgroundLang("zh-CN")

	cases := map[string]Lang{
		"en-US":   EnUS,
		"zh-CN":   ZhCN,
		" en-US ": EnUS, // 库里存进空格也认
		"":        ZhCN, // 空 = 跟随站点设置
		"de-DE":   ZhCN, // 不支持的语言不该露出原值
	}
	for pref, want := range cases {
		if got := ResolveLang(pref); got != want {
			t.Errorf("ResolveLang(%q) = %v, want %v", pref, got, want)
		}
	}

	// 站点语言换成英文后，"跟随设置"的用户也应拿到英文
	SetBackgroundLang("en-US")
	if got := ResolveLang(""); got != EnUS {
		t.Errorf("空偏好应跟随站点语言，实际 %v", got)
	}
}

// TestInboxTitlesRenderPerUser 站内通知标题按收件人语言渲染
//
// 这条链路的价值就在于「同一条工单事件，中文用户和英文用户各看各的」，
// 用带参数的标题验证格式化没在翻译时丢参数。
func TestInboxTitlesRenderPerUser(t *testing.T) {
	zh := Uf("zh-CN", "inbox.mentioned_you", "张三", "OPS-1")
	en := Uf("en-US", "inbox.mentioned_you", "Alice", "OPS-1")

	if zh != "张三 在工单 OPS-1 中提及了您" {
		t.Errorf("中文标题不符: %q", zh)
	}
	if en != "Alice mentioned you on issue OPS-1" {
		t.Errorf("英文标题不符: %q", en)
	}
	// 格式化占位符数量必须与调用方传的参数对得上，否则会渲染出 %!s(MISSING)
	for _, s := range []string{zh, en} {
		if strings.Contains(s, "%!") {
			t.Errorf("参数与占位符不匹配: %q", s)
		}
	}
}

// TestNotifyAndInboxKeysFormatConsistently
// 中英两版的格式化占位符数量必须一致，否则切到英文就会出现 %!s(MISSING)
func TestNotifyAndInboxKeysFormatConsistently(t *testing.T) {
	count := func(s string) int { return strings.Count(s, "%s") + strings.Count(s, "%d") }
	for _, key := range Keys() {
		zh, en := messages[key][ZhCN], messages[key][EnUS]
		if count(zh) != count(en) {
			t.Errorf("%s 的占位符数量不一致: zh=%q en=%q", key, zh, en)
		}
	}
}

// TestNoStrayFormatVerbs 文案里不能出现非法的 % 序列
//
// 后端文案最终都过 fmt.Sprintf。写一个字面量百分号（"完成率 50%"）
// 会被当成格式动词，实际渲染出 "50%!(NOVERB)" 之类的东西直接给到用户。
//
// 上面那个数量一致性测试拦不住这种：中英两边都写错时数量仍然相等。
// 字面量百分号要写成 %%。
func TestNoStrayFormatVerbs(t *testing.T) {
	// fmt 支持的动词，加上转义的 %%
	const validVerbs = "sdvwqtfgexXbcopUT%"
	for _, key := range Keys() {
		for lang, text := range messages[key] {
			for i := 0; i < len(text); i++ {
				if text[i] != '%' {
					continue
				}
				if i+1 >= len(text) {
					t.Errorf("%s[%s] 以孤立的 %% 结尾: %q", key, lang, text)
					break
				}
				next := text[i+1]
				if !strings.ContainsRune(validVerbs, rune(next)) {
					t.Errorf("%s[%s] 含非法格式动词 %%%c（字面量百分号要写成 %%%%）: %q", key, lang, next, text)
				}
				if next == '%' {
					i++ // 跳过转义对，避免把 %% 的后一个 % 再判一次
				}
			}
		}
	}
}

// TestSetBackgroundLangIsRaceFree 后台语言可在运行时被管理员改写
//
// 管理员在系统设置里保存语言是一次写入，而此刻发通知的 goroutine 正在读。
// 换成原子指针之前，这个测试在 -race 下必然报 DATA RACE。
func TestSetBackgroundLangIsRaceFree(t *testing.T) {
	orig := BackgroundLang()
	defer SetBackgroundLang(string(orig))

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// 读侧：模拟并发发通知
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					// 取到的必须是两种语言之一，不能是空串或撕裂的值
					if got := B("notify.label_status"); got != "状态" && got != "Status" {
						t.Errorf("读到非法文案: %q", got)
						return
					}
				}
			}
		}()
	}

	// 写侧：模拟管理员反复切换
	for range 200 {
		SetBackgroundLang("en-US")
		SetBackgroundLang("zh-CN")
	}
	close(stop)
	wg.Wait()
}

// TestSetBackgroundLangKeepsCurrentOnGarbage 无法识别的值保持原设置
//
// 配置里写错一个字母就把整站语言悄悄切回中文，比保持原样更难排查。
func TestSetBackgroundLangKeepsCurrentOnGarbage(t *testing.T) {
	orig := BackgroundLang()
	defer SetBackgroundLang(string(orig))

	SetBackgroundLang("en-US")
	for _, junk := range []string{"", "  ", "en", "zh", "EN-US", "de-DE", "null"} {
		SetBackgroundLang(junk)
		if got := BackgroundLang(); got != EnUS {
			t.Errorf("SetBackgroundLang(%q) 把语言改成了 %v，应保持 en-US", junk, got)
		}
	}
}

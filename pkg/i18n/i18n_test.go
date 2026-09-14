package i18n

import (
	"context"
	"strings"
	"testing"
)

// TestAllKeysTranslated 每个 key 必须在所有支持的语言里都有非空文案。
//
// 缺失时 TLang 会回落到中文——界面不报错，但英文用户会突然看到一句中文，
// 而且没有任何地方提示。这里把这类漂移变成测试失败。
func TestAllKeysTranslated(t *testing.T) {
	var missing []string
	for _, key := range Keys() {
		for _, lang := range Langs() {
			if strings.TrimSpace(messages[key][lang]) == "" {
				missing = append(missing, key+" / "+string(lang))
			}
		}
	}
	if len(missing) > 0 {
		t.Fatalf("以下 key 缺少翻译:\n  %s", strings.Join(missing, "\n  "))
	}
}

// TestEnglishIsNotChinese 英文文案里不该混入中文字符。
// 批量补翻译时很容易漏掉某条、直接复制中文过去，肉眼很难发现。
func TestEnglishIsNotChinese(t *testing.T) {
	for _, key := range Keys() {
		en := messages[key][EnUS]
		for _, r := range en {
			if r >= 0x4e00 && r <= 0x9fff {
				t.Errorf("英文文案含中文字符: %s = %q", key, en)
				break
			}
		}
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	cases := map[string]Lang{
		"":                        DefaultLang,
		"zh-CN,zh;q=0.9":          ZhCN,
		"zh":                      ZhCN,
		"en-US,en;q=0.9":          EnUS,
		"en":                      EnUS,
		"en-GB":                   EnUS,
		"fr-FR,fr;q=0.9":          DefaultLang, // 不支持的语言回落
		"fr-FR,fr;q=0.9,en;q=0.8": EnUS,        // 跳过不支持的，取第一个能识别的
		"  EN-us , zh ;q=0.5":     EnUS,        // 大小写与空格容错
	}
	for header, want := range cases {
		if got := ParseAcceptLanguage(header); got != want {
			t.Errorf("ParseAcceptLanguage(%q) = %v, want %v", header, got, want)
		}
	}
}

func TestContextRoundTrip(t *testing.T) {
	ctx := WithLang(context.Background(), EnUS)
	if got := FromContext(ctx); got != EnUS {
		t.Fatalf("FromContext = %v, want %v", got, EnUS)
	}
	// 没有语言时回落到默认，而不是空串
	if got := FromContext(context.Background()); got != DefaultLang {
		t.Fatalf("空 context 应回落到 %v，实际 %v", DefaultLang, got)
	}
	//nolint:staticcheck // SA1012: 显式测试 nil context 的健壮性
	if got := FromContext(nil); got != DefaultLang {
		t.Fatalf("nil context 应回落到 %v，实际 %v", DefaultLang, got)
	}
}

func TestTranslatesByContext(t *testing.T) {
	zh := T(WithLang(context.Background(), ZhCN), "issue.not_found")
	en := T(WithLang(context.Background(), EnUS), "issue.not_found")

	if zh != "工单不存在" {
		t.Errorf("中文文案不符: %q", zh)
	}
	if en != "Issue not found" {
		t.Errorf("英文文案不符: %q", en)
	}
	// 未知 key 原样返回，便于定位而不是静默显示空白
	if got := T(context.Background(), "nope.missing"); got != "nope.missing" {
		t.Errorf("未知 key 应原样返回，实际 %q", got)
	}
}

// ---- 校验错误本地化 ----

type loginForm struct {
	Username string `validate:"required,min=3"`
	Password string `validate:"required,min=6"`
	Email    string `validate:"omitempty,email"`
}

func validateForm(t *testing.T, f loginForm) error {
	t.Helper()
	return newTestValidator().Struct(f)
}

// TestValidationErrorIsHuman 校验错误必须翻成人话，
// 不能把 validator 的原始英文（含结构体路径）直接抛给用户。
func TestValidationErrorIsHuman(t *testing.T) {
	err := validateForm(t, loginForm{Username: "ab", Password: "123456"})
	if err == nil {
		t.Fatal("期望校验失败")
	}

	zh := ValidationError(WithLang(context.Background(), ZhCN), err)
	en := ValidationError(WithLang(context.Background(), EnUS), err)

	for _, bad := range []string{"loginForm", "Field validation", "tag"} {
		if strings.Contains(zh, bad) || strings.Contains(en, bad) {
			t.Errorf("不应暴露 validator 内部信息 %q:\n  zh=%q\n  en=%q", bad, zh, en)
		}
	}
	if !strings.Contains(zh, "用户名") {
		t.Errorf("中文应使用字段的用户可读名称: %q", zh)
	}
	if !strings.Contains(en, "Username") {
		t.Errorf("英文应使用字段的用户可读名称: %q", en)
	}
	for _, r := range en {
		if r >= 0x4e00 && r <= 0x9fff {
			t.Errorf("英文校验提示混入中文: %q", en)
			break
		}
	}
}

// TestValidationErrorFallsBackForNonValidator
// JSON 语法错误这类非 validator 错误不该泄露解析器内部信息
func TestValidationErrorFallsBackForNonValidator(t *testing.T) {
	got := ValidationError(WithLang(context.Background(), EnUS), errStub{})
	if got != TLang(EnUS, "common.bad_request") {
		t.Fatalf("非 validator 错误应回落到通用提示，实际 %q", got)
	}
	if got := ValidationError(context.Background(), nil); got == "" {
		t.Fatal("nil error 不应返回空串")
	}
}

type errStub struct{}

func (errStub) Error() string { return "json: cannot unmarshal number into field X of type string" }

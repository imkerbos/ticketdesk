package i18n

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSentinelsAreNotWrapped 携带语言包 key 的哨兵错误不能被 fmt.Errorf 包裹。
//
// 由来：response.Error 靠"整串是不是已登记的 key"来决定翻不翻译。
// 一旦哨兵被包进更长的字符串（fmt.Errorf("xxx: %w", ErrSomething)），
// 整串就不再是 key —— 既翻译不了，还会把 "issue.bad_time_format"
// 这种裸 key 原样甩给用户。这个测试静态扫描源码，把这类写法挡在提交前。
//
// 真实案例：工时接口曾返回 "invalid worked_at format: issue.bad_time_format"。
func TestSentinelsAreNotWrapped(t *testing.T) {
	root := findRepoRoot(t)

	// 收集所有值是已登记 key 的哨兵错误变量名
	sentinelDecl := regexp.MustCompile(`(\w+)\s*=\s*errors\.New\("([a-z0-9_.]+)"\)`)
	sentinels := map[string]string{}
	walkGoFiles(t, root, func(path, src string) {
		for _, m := range sentinelDecl.FindAllStringSubmatch(src, -1) {
			if Has(m[2]) {
				sentinels[m[1]] = m[2]
			}
		}
	})
	if len(sentinels) == 0 {
		t.Fatal("没扫到任何携带 key 的哨兵错误，测试本身可能失效了")
	}

	var bad []string
	walkGoFiles(t, root, func(path, src string) {
		for i, line := range strings.Split(src, "\n") {
			if !strings.Contains(line, "%w") {
				continue
			}
			for name, key := range sentinels {
				if regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`).MatchString(line) {
					bad = append(bad, filepath.Base(path)+":"+itoa(i+1)+
						" 包裹了 "+name+"（key: "+key+"）\n    "+strings.TrimSpace(line))
				}
			}
		}
	})
	if len(bad) > 0 {
		t.Errorf("以下位置把携带语言包 key 的哨兵错误包进了更长的字符串，\n"+
			"会导致响应层认不出 key、把裸 key 抛给用户。\n"+
			"改法：直接返回哨兵，附加细节写日志。\n  %s", strings.Join(bad, "\n  "))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// findRepoRoot 从当前包向上找到含 go.mod 的目录
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("找不到仓库根目录")
	return ""
}

func walkGoFiles(t *testing.T, root string, fn func(path, src string)) {
	t.Helper()
	for _, sub := range []string{"internal", "pkg"} {
		err := filepath.Walk(filepath.Join(root, sub), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fn(path, string(b))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

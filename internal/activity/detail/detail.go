// Package detail 构造活动日志的结构化详情。
//
// 活动是持久化的：写入时的界面语言不等于读取时的界面语言。
// 张三用中文改了工单、李四用英文看活动流，如果在后端按写入方的 locale 渲染，
// 就把张三的语言烧进了数据库 —— 那只是把「写死中文」换个语言重犯一遍。
//
// 所以这里只存「说什么」和「填什么」，翻译交给前端在渲染时做
// （见 web/src/utils/activity.ts 的 formatActivityDetails）。
package detail

import (
	"encoding/json"

	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/pkg/logger"
)

// Detail 是活动详情的结构化形式：Key 指向语言包里的一条文案，Params 是它的插值。
type Detail struct {
	Key    string         `json:"key"`
	Params map[string]any `json:"params,omitempty"`
	// Keys 里的值本身是语言包 key，前端插值前会先翻译一层。
	// 用于「节点 X (通过)」这种句子里嵌枚举值的情况。
	//
	// 单独开一个字段而不是靠参数名后缀约定（比如「叫 xxxKey 的就翻译」）——
	// 那种约定会和 issueKey、projectKey 这类真实字段名撞上。
	Keys map[string]string `json:"keys,omitempty"`
	// Items 用于「更新了 A、B、C」这类由若干条子文案拼成的详情。
	// 拼接顺序和分隔符由前端按语言决定，后端不参与排版。
	Items []Detail `json:"items,omitempty"`
}

// New 构造一条详情并序列化成可直接存库的字符串。
//
// params 传键值对，个数必须成双，例如 New("issue.created", "title", issue.Title)。
// 序列化失败时退回 key 本身 —— 活动日志不该因为详情拼不出来就整条丢掉。
func New(key string, params ...any) string {
	return marshal(Detail{Key: key, Params: toParams(params)})
}

// NewWithKeys 和 New 一样，另外带一组「值本身是语言包 key」的参数。
func NewWithKeys(key string, keys map[string]string, params ...any) string {
	return marshal(Detail{Key: key, Keys: keys, Params: toParams(params)})
}

// List 构造一条由多个子项拼成的详情，例如「更新了: 标题, 优先级 → P1」。
func List(key string, items []Detail) string {
	if len(items) == 0 {
		return ""
	}
	return marshal(Detail{Key: key, Items: items})
}

// Item 构造 List 里的一个子项。
func Item(key string, params ...any) Detail {
	return Detail{Key: key, Params: toParams(params)}
}

func toParams(kv []any) map[string]any {
	if len(kv) == 0 {
		return nil
	}
	// 个数不成双说明调用点写错了，丢掉尾巴而不是 panic —— 记日志不值得让请求挂掉
	if len(kv)%2 != 0 {
		logger.Warn("activity detail params not in pairs, dropping the last one",
			zap.Int("count", len(kv)))
		kv = kv[:len(kv)-1]
	}
	params := make(map[string]any, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		name, ok := kv[i].(string)
		if !ok {
			logger.Warn("activity detail param name is not a string, skipped")
			continue
		}
		params[name] = kv[i+1]
	}
	return params
}

func marshal(d Detail) string {
	b, err := json.Marshal(d)
	if err != nil {
		logger.Warn("failed to marshal activity detail, falling back to the key",
			zap.String("key", d.Key), zap.Error(err))
		return d.Key
	}
	return string(b)
}

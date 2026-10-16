// 本文件解决「换源之后中文名从哪来」。
//
// 新数据源（helldivers-api）抓的是英文 wiki，装备名只有英文；而群里的查询与卡片标题都靠中文名。
// 中文名分三层，前一层命中就不看后面：
//
//  1. 本地覆盖 data/arsenal/zh-names.local.json（id → 中文名）：留一个不改代码就能补名字的口子；
//  2. 内置表 src/arsenal/data/zh-names.json（随二进制发布）：298 件取 HD2Tool 目录里的官方简中名，
//     新装备取社区中文维基的术语表；
//  3. 运行期术语表：源每次刷新时顺手拉一遍社区中文维基的 terms.json（英文名 → 中文名），
//     这样游戏更新出新品时，名字不用等我们发版。
//
// 三层都没有就留空，由 ParseCatalog 回退成英文名——宁可显示英文，也不猜一个错的中文名。
package arsenal

import (
	"embed"
	"encoding/json"
	"os"
	"strings"
)

//go:embed data/zh-names.json
var zhNameFiles embed.FS

// embeddedZhNameFile 是内置中文名表的文件名。
const embeddedZhNameFile = "data/zh-names.json"

// zhNamesLocalFile 是本地覆盖表的文件名（放在装备缓存目录下，不进仓库）。
const zhNamesLocalFile = "zh-names.local.json"

// zhTables 是中文名表文件的结构（内置表与本地覆盖表共用）。
type zhTables struct {
	Note     string            `json:"note"`
	Items    map[string]string `json:"items"`
	Warbonds map[string]string `json:"warbonds"`
}

// zhResolver 把三层中文名合成一次查询。
type zhResolver struct {
	itemByID      map[string]string
	warbondByID   map[string]string
	itemByEnglish map[string]string
	// warbondByEnglish 目前只有术语表能提供（债券英文名 → 中文名）。
	warbondByEnglish map[string]string
}

// newZhResolver 构造函数：local 是本地覆盖表，glossary 是运行期术语表（英文名 → 中文名）。
func newZhResolver(local zhTables, glossary map[string]string) *zhResolver {
	embedded := loadEmbeddedZhTables()
	resolver := &zhResolver{
		itemByID:         mergeZhMaps(embedded.Items, local.Items),
		warbondByID:      mergeZhMaps(embedded.Warbonds, local.Warbonds),
		itemByEnglish:    map[string]string{},
		warbondByEnglish: map[string]string{},
	}
	for english, zh := range glossary {
		if key := normalizeZhNameKey(english); key != "" {
			resolver.itemByEnglish[key] = zh
			resolver.warbondByEnglish[key] = zh
		}
	}
	return resolver
}

// Item 返回一件装备的中文名；查不到返回空串（调用方回退英文名）。
func (r *zhResolver) Item(id, nameEn string) string {
	if r == nil {
		return ""
	}
	if zh := strings.TrimSpace(r.itemByID[id]); zh != "" {
		return zh
	}
	return r.itemByEnglish[normalizeZhNameKey(nameEn)]
}

// Warbond 返回一本债券的中文名；查不到返回空串（调用方回退英文名）。
func (r *zhResolver) Warbond(id, nameEn string) string {
	if r == nil {
		return ""
	}
	if zh := strings.TrimSpace(r.warbondByID[id]); zh != "" {
		return zh
	}
	return r.warbondByEnglish[normalizeZhNameKey(nameEn)]
}

// loadEmbeddedZhTables 解析内置中文名表；文件坏了就返回空表（顶多少几个中文名，不该让目录失败）。
func loadEmbeddedZhTables() zhTables {
	raw, err := zhNameFiles.ReadFile(embeddedZhNameFile)
	if err != nil {
		return zhTables{}
	}
	var tables zhTables
	if err := json.Unmarshal(raw, &tables); err != nil {
		return zhTables{}
	}
	return tables
}

// loadLocalZhTables 读本地覆盖表；文件不存在（最常见）或格式不对都返回空表。
func loadLocalZhTables(path string) zhTables {
	if strings.TrimSpace(path) == "" {
		return zhTables{}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return zhTables{}
	}
	var tables zhTables
	if err := json.Unmarshal(raw, &tables); err != nil {
		return zhTables{}
	}
	return tables
}

// mergeZhMaps 合并两张表，overlay 里的非空值覆盖 base。
func mergeZhMaps(base, overlay map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(overlay))
	for id, zh := range base {
		if trimmed := strings.TrimSpace(zh); trimmed != "" {
			out[id] = trimmed
		}
	}
	for id, zh := range overlay {
		if trimmed := strings.TrimSpace(zh); trimmed != "" && strings.TrimSpace(id) != "" {
			out[strings.TrimSpace(id)] = trimmed
		}
	}
	return out
}

// glossaryEntry 是社区中文维基术语表（terms.json）里的一条。
type glossaryEntry struct {
	En string `json:"en"`
	Zh string `json:"zh"`
}

// glossaryFile 是 terms.json 的解析目标；它只声明用得上的两个字段。
type glossaryFile struct {
	UpdatedAt string          `json:"updated_at"`
	Terms     []glossaryEntry `json:"terms"`
}

// parseGlossary 把术语表原文转成「英文名 → 中文名」；解析不出来返回 nil。
func parseGlossary(raw []byte) map[string]string {
	var parsed glossaryFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil
	}
	return parseGlossaryTerms(parsed)
}

// parseGlossaryTerms 把术语表转成「英文名 → 中文名」；同名取第一条（术语表按来源分组，先出现的是武器/债券）。
func parseGlossaryTerms(parsed glossaryFile) map[string]string {
	out := make(map[string]string, len(parsed.Terms))
	for _, term := range parsed.Terms {
		key := normalizeZhNameKey(term.En)
		zh := strings.TrimSpace(term.Zh)
		if key == "" || zh == "" {
			continue
		}
		if _, exists := out[key]; !exists {
			out[key] = zh
		}
	}
	return out
}

// normalizeZhNameKey 归一化英文名，让「上游写法」与「术语表写法」能对上：
// 折叠大小写与空白，弯引号统一成直引号，去掉零宽字符。
func normalizeZhNameKey(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.NewReplacer(
		"\u2019", "'", "\u2018", "'",
		"\u201c", "\"", "\u201d", "\"",
		"\u200b", "", "\u00a0", " ",
	).Replace(trimmed)
	return strings.ToLower(strings.Join(strings.Fields(trimmed), " "))
}

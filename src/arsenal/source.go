// 本文件是装备目录的「从哪来」：把不同上游统一成一份 HD2Tool 形态的 catalog JSON 交给 Store。
//
// 为什么统一成这个形态：目录的解析、索引、检索、缓存与降级只写一份，换源只影响
// 「这份 JSON 从哪来」，Store 与插件层一行都不用动；磁盘快照也天然还是同一种文件，
// 换源时旧快照照样能当降级数据用。
package arsenal

import (
	"context"
	"strings"
)

// Source 是装备目录的来源。
type Source interface {
	// Name 是源的短名，只用于日志与排障。
	Name() string
	// CatalogJSON 取回（或现场生成）一份目录 JSON；失败要让调用方能够降级到磁盘快照。
	CatalogJSON(ctx context.Context) ([]byte, error)
	// AliasesJSON 取回玩家外号表；返回 nil 表示这个源不提供外号表（保留磁盘上的旧文件）。
	AliasesJSON(ctx context.Context) []byte
	// ImageURLs 把 Item.ImagePath 展开成候选下载地址（主源在前，镜像在后）。
	ImageURLs(imagePath string) []string
}

// hd2ToolSource 是建站时的老源：SalmonC/HD2Tool 仓库里的静态目录。
//
// 它已经停止更新（数据最后一批是 2026-08-14，见 README 的「数据来源」），默认不再使用，
// 但保留实现：一是新源哪天挂了可以直接切回来，二是两种源并存时磁盘快照格式一致。
type hd2ToolSource struct{ store *Store }

func (s *hd2ToolSource) Name() string { return "hd2tool" }

// CatalogJSON 直接下载上游的 catalog.json（镜像可用时走镜像）。
func (s *hd2ToolSource) CatalogJSON(ctx context.Context) ([]byte, error) {
	return s.store.fetch(ctx, s.store.rawURLs("src/data/catalog.json"), maxCatalogBytes)
}

// AliasesJSON 下载社区外号表；失败只记日志，当作「这个源这轮没给外号」。
func (s *hd2ToolSource) AliasesJSON(ctx context.Context) []byte {
	raw, err := s.store.fetch(ctx, s.store.rawURLs("src/data/community-aliases.json"), maxCatalogBytes)
	if err != nil {
		s.store.log("[arsenal] 下载玩家外号表失败（少几条别名，不影响其它功能）：%v", err)
		return nil
	}
	return raw
}

// ImageURLs 把仓库内的相对路径展开成「主源 + 镜像」。
func (s *hd2ToolSource) ImageURLs(imagePath string) []string {
	return s.store.rawURLs("public/" + imagePath)
}

// newSource 按配置挑一个来源。取值不认识时退回默认源并记一条日志：
// 配置写错不该让整个图鉴命令全挂，但也不能悄悄什么都不说。
func newSource(store *Store) Source {
	switch strings.ToLower(strings.TrimSpace(store.cfg.Source)) {
	case sourceKindHD2Tool:
		return &hd2ToolSource{store: store}
	case "", sourceKindHelldiversAPI:
		return newAPISource(store)
	default:
		store.log("[arsenal] arsenal.source=%q 不认识，按 %s 处理", store.cfg.Source, sourceKindHelldiversAPI)
		return newAPISource(store)
	}
}

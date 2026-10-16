// 中文名解析的用例：三层优先级、归一化规则，以及内置表本身的可用性。
package arsenal

import "testing"

// TestEmbeddedZhTableCoversCatalog 校验内置中文名表真的打进了二进制、且覆盖到老目录的规模。
func TestEmbeddedZhTableCoversCatalog(t *testing.T) {
	tables := loadEmbeddedZhTables()
	if len(tables.Items) < 300 {
		t.Fatalf("内置中文名表太少（%d 条）：数据文件可能没打进来", len(tables.Items))
	}
	if len(tables.Warbonds) < 24 {
		t.Errorf("内置债券中文名太少：%d 条", len(tables.Warbonds))
	}
	for id, want := range map[string]string{
		"ar-23-liberator":  "解放者",
		"ar-11-arbitrator": "AR-11 仲裁者",
		"a-35-recon":       "A-35“侦察者”",
	} {
		if got := tables.Items[id]; got != want {
			t.Errorf("%s 的中文名应为 %q，实际 %q", id, want, got)
		}
	}
	if got := tables.Warbonds["ironclad-democracy"]; got != "民主铁壁" {
		t.Errorf("新债券的中文名应由社区术语表补上，实际 %q", got)
	}
}

// TestZhResolverPrecedence 校验三层优先级：本地覆盖 > 内置表 > 运行期术语表。
func TestZhResolverPrecedence(t *testing.T) {
	local := zhTables{
		Items:    map[string]string{"ar-23-liberator": "本地译名", "custom-gun": "自定义枪"},
		Warbonds: map[string]string{"ironclad-democracy": "本地债券名"},
	}
	glossary := map[string]string{
		"AR-23 Liberator":    "术语表译名",
		"Ironclad Democracy": "术语表债券名",
		"AR-11 Arbitrator":   "术语表仲裁者",
	}
	resolver := newZhResolver(local, glossary)

	cases := []struct {
		name     string
		got      string
		expected string
	}{
		{"本地覆盖优先", resolver.Item("ar-23-liberator", "AR-23 Liberator"), "本地译名"},
		{"内置表次之", resolver.Item("a-35-recon", "A-35 Recon"), "A-35“侦察者”"},
		{"术语表补缺", resolver.Item("ar-11-arbitrator", "AR-11 Arbitrator"), "AR-11 仲裁者"},
		{"只有本地有的条目", resolver.Item("custom-gun", "Custom Gun"), "自定义枪"},
		{"都没有返回空串", resolver.Item("unknown-gun", "Unknown Gun"), ""},
		{"债券同理（本地优先）", resolver.Warbond("ironclad-democracy", "Ironclad Democracy"), "本地债券名"},
		{"债券查不到返回空串", resolver.Warbond("unknown-book", "Unknown Book"), ""},
	}
	for _, c := range cases {
		if c.got != c.expected {
			t.Errorf("%s：期望 %q，实际 %q", c.name, c.expected, c.got)
		}
	}
}

// TestZhResolverWithoutGlossary 校验术语表缺失（nil）时只靠内置表也能工作。
func TestZhResolverWithoutGlossary(t *testing.T) {
	resolver := newZhResolver(zhTables{}, nil)
	if got := resolver.Item("g-4-gas", "G-4 Gas"); got != "毒气榴弹" {
		t.Errorf("内置表应命中：%q", got)
	}
	if got := resolver.Item("ar-11-arbitrator", "AR-11 Arbitrator"); got != "AR-11 仲裁者" {
		t.Errorf("内置表里已有新装备译名：%q", got)
	}
}

// TestZhResolverNilSafety 校验零值 resolver 不会崩：调用方拿不到数据层时也会走到这里。
func TestZhResolverNilSafety(t *testing.T) {
	var resolver *zhResolver
	if got := resolver.Item("ar-23-liberator", "AR-23 Liberator"); got != "" {
		t.Errorf("nil resolver 应返回空串，实际 %q", got)
	}
	if got := resolver.Warbond("ironclad-democracy", "Ironclad Democracy"); got != "" {
		t.Errorf("nil resolver 应返回空串，实际 %q", got)
	}
}

// TestNormalizeZhNameKey 校验英文名归一化：大小写、空白、弯引号、零宽字符都要能对上。
func TestNormalizeZhNameKey(t *testing.T) {
	cases := map[string]string{
		"  AR-23   Liberator ":    "ar-23 liberator",
		"Castellan’s Creed":       "castellan's creed",
		"Castellan's Creed":       "castellan's creed",
		"MG‑43\u200b Machine Gun": "mg‑43 machine gun",
		"":                        "",
		"   ":                     "",
	}
	for input, want := range cases {
		if got := normalizeZhNameKey(input); got != want {
			t.Errorf("normalizeZhNameKey(%q) = %q，期望 %q", input, got, want)
		}
	}
}

// TestParseGlossaryKeepsFirstEntry 校验术语表同名条目取第一条，且坏数据被跳过。
func TestParseGlossaryKeepsFirstEntry(t *testing.T) {
	raw := []byte(`{
	  "updated_at": "2026-10-03T12:53:02Z",
	  "terms": [
	    {"en": "AR-11 Arbitrator", "zh": "AR-11 仲裁者"},
	    {"en": "AR-11 Arbitrator", "zh": "另一个译名"},
	    {"en": "", "zh": "没有英文名"},
	    {"en": "G-8 Immolation", "zh": ""},
	    {"en": "G-8 Immolation", "zh": "G-8 燔祭"}
	  ]
	}`)
	glossary := parseGlossary(raw)
	if len(glossary) != 2 {
		t.Fatalf("应只有两条有效条目，实际 %d：%v", len(glossary), glossary)
	}
	if glossary["ar-11 arbitrator"] != "AR-11 仲裁者" {
		t.Errorf("同名条目应取第一条：%q", glossary["ar-11 arbitrator"])
	}
	if glossary["g-8 immolation"] != "G-8 燔祭" {
		t.Errorf("空中文名应被跳过：%q", glossary["g-8 immolation"])
	}
}

// TestParseGlossaryToleratesBrokenJSON 校验术语表坏掉时返回 nil 而不是 panic。
func TestParseGlossaryToleratesBrokenJSON(t *testing.T) {
	if got := parseGlossary([]byte("{ 这不是 JSON")); got != nil {
		t.Errorf("坏 JSON 应返回 nil，实际 %v", got)
	}
	if got := parseGlossary(nil); got != nil {
		t.Errorf("空内容应返回 nil，实际 %v", got)
	}
}

// TestLoadLocalZhTablesMissingFile 校验本地覆盖表缺失/坏掉都只是「没有覆盖」。
func TestLoadLocalZhTablesMissingFile(t *testing.T) {
	if got := loadLocalZhTables(""); len(got.Items) != 0 || len(got.Warbonds) != 0 {
		t.Errorf("空路径应给空表：%+v", got)
	}
	if got := loadLocalZhTables(t.TempDir() + "/没有这个文件.json"); len(got.Items) != 0 {
		t.Errorf("文件缺失应给空表：%+v", got)
	}
}

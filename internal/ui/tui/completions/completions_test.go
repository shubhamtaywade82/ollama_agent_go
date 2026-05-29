package completions

import "testing"

func TestSlashProviderTrigger(t *testing.T) {
	p := SlashProvider{Commands: []Command{{Name: "/model"}, {Name: "/help"}}}
	cases := []struct {
		line      string
		cursor    int
		wantOK    bool
		wantQuery string
		wantStart int
	}{
		{"/mod", 4, true, "mod", 0},
		{"/", 1, true, "", 0},
		{"/model foo", 10, false, "", 0}, // past command name
		{"hello", 5, false, "", 0},       // no slash
		{"/help", 3, true, "he", 0},      // cursor mid-token
	}
	for _, c := range cases {
		ok, q, start := p.Trigger(c.line, c.cursor)
		if ok != c.wantOK || q != c.wantQuery || start != c.wantStart {
			t.Errorf("Trigger(%q,%d) = (%v,%q,%d), want (%v,%q,%d)",
				c.line, c.cursor, ok, q, start, c.wantOK, c.wantQuery, c.wantStart)
		}
	}
}

func TestSlashProviderItemsRanked(t *testing.T) {
	p := SlashProvider{Commands: []Command{{Name: "/model"}, {Name: "/metrics"}, {Name: "/help"}}}
	items := p.Items("mo")
	if len(items) == 0 || items[0].Insert != "/model" {
		t.Fatalf("expected /model first, got %+v", items)
	}
	if items[0].Title != "/model" {
		t.Errorf("title should show slash, got %q", items[0].Title)
	}
}

func TestTokenAt(t *testing.T) {
	cases := []struct {
		line      string
		cursor    int
		marker    rune
		wantOK    bool
		wantQuery string
		wantStart int
	}{
		{"@sr", 3, '@', true, "sr", 0},
		{"see @foo", 8, '@', true, "foo", 4},
		{"see @foo bar", 12, '@', false, "", 0}, // token ended by space
		{"email@host", 10, '@', false, "", 0},   // not preceded by space
		{"#skill", 6, '#', true, "skill", 0},
		{"a #s", 4, '#', true, "s", 2},
	}
	for _, c := range cases {
		ok, q, start := tokenAt(runesBefore(c.line, c.cursor), c.marker)
		if ok != c.wantOK || q != c.wantQuery || start != c.wantStart {
			t.Errorf("tokenAt(%q,%d,%q) = (%v,%q,%d), want (%v,%q,%d)",
				c.line, c.cursor, string(c.marker), ok, q, start, c.wantOK, c.wantQuery, c.wantStart)
		}
	}
}

func TestModelProviderTrigger(t *testing.T) {
	p := ModelProvider{Names: func() []string { return []string{"qwen3.5:4b", "llama3"} }}
	ok, q, start := p.Trigger("/model qw", 9)
	if !ok || q != "qw" || start != 7 {
		t.Fatalf("got (%v,%q,%d), want (true,qw,7)", ok, q, start)
	}
	if ok, _, _ := p.Trigger("/model qwen extra", 17); ok {
		t.Error("should not trigger past the model token")
	}
	items := p.Items("qw")
	if len(items) != 1 || items[0].Insert != "/model qwen3.5:4b" {
		t.Fatalf("bad items: %+v", items)
	}
}

func TestAcceptSplice(t *testing.T) {
	m := New(SlashProvider{Commands: []Command{{Name: "/model"}}})
	m.Refresh("/mod", 4)
	if !m.Visible() {
		t.Fatal("menu should be visible")
	}
	line, cur, ok := m.Accept("/mod", 4)
	if !ok || line != "/model" || cur != len("/model") {
		t.Fatalf("Accept = (%q,%d,%v), want (/model,6,true)", line, cur, ok)
	}
	if m.Visible() {
		t.Error("menu should hide after accept")
	}
}

func TestAcceptSpliceMidLine(t *testing.T) {
	m := New(FileProvider{Root: "."})
	// Simulate an active file token "@me" at offset 4 inside "see @me now".
	m.active = FileProvider{}
	m.items = []Item{{Title: "menu.go", Insert: "@menu.go"}}
	m.start = 4
	m.idx = 0
	m.visible = true
	line, cur, ok := m.Accept("see @me now", 7)
	if !ok || line != "see @menu.go now" || cur != len("see @menu.go") {
		t.Fatalf("Accept = (%q,%d,%v), want (see @menu.go now,12,true)", line, cur, ok)
	}
}

func TestRefreshHidesWhenNoItems(t *testing.T) {
	m := New(SlashProvider{Commands: []Command{{Name: "/model"}}})
	m.Refresh("hello world", 11) // no provider active
	if m.Visible() {
		t.Error("menu should be hidden for plain text")
	}
}

func TestMoveClamps(t *testing.T) {
	m := New(SlashProvider{Commands: []Command{{Name: "/a"}, {Name: "/b"}, {Name: "/c"}}})
	m.Refresh("/", 1)
	m.Move(-1)
	if m.idx != 0 {
		t.Errorf("idx should clamp to 0, got %d", m.idx)
	}
	m.Move(100)
	if m.idx != len(m.items)-1 {
		t.Errorf("idx should clamp to last, got %d", m.idx)
	}
}

package executor

import (
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"simple", "'simple'"},
		{"/data/proj backend", "'/data/proj backend'"},
		{"it's", `'it'\''s'`},
		{`say "hi" $HOME ` + "`whoami`", `'say "hi" $HOME ` + "`whoami`" + `'`},
		{"line1\nline2", "'line1\nline2'"},
		{"", "''"},
	}
	for _, c := range cases {
		if got := ShellQuote(c.in); got != c.want {
			t.Errorf("ShellQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEnvExportPrefix(t *testing.T) {
	if got := EnvExportPrefix(nil); got != "" {
		t.Errorf("nil env should produce empty prefix, got %q", got)
	}
	out := EnvExportPrefix(map[string]string{
		"B": "two",
		"A": "it's \"quoted\" $x",
	})
	// 键排序稳定
	want := "export A='it'\\''s \"quoted\" $x'\nexport B='two'\n"
	if out != want {
		t.Errorf("EnvExportPrefix = %q, want %q", out, want)
	}
	if !strings.HasPrefix(out, "export A=") {
		t.Errorf("keys should be sorted: %q", out)
	}
}

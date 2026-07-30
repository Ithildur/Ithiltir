package theme

import (
	"strings"
	"testing"
)

func TestBuiltinCSSCanonicalizesOperatorTheme(t *testing.T) {
	css, err := BuiltinCSS("operator")
	if err != nil {
		t.Fatalf("BuiltinCSS(operator): %v", err)
	}

	out := string(css)
	if !strings.Contains(out, "--theme-fg-accent: #1e7f42;") {
		t.Fatalf("expected operator theme tokens in output, got:\n%s", out)
	}
	if strings.Contains(out, "data-theme='operator'") || strings.Contains(out, `data-theme="operator"`) {
		t.Fatalf("expected active css to drop theme-specific selectors, got:\n%s", out)
	}
}

func TestBuildActiveCSS(t *testing.T) {
	tests := []struct {
		name    string
		css     string
		wantErr string
		want    []string
	}{
		{
			name:    "unexpected selector",
			css:     ".bad { --theme-fg-default: #fff; }",
			wantErr: "unsupported selector",
		},
		{
			name:    "standard property",
			css:     ":root[data-theme='custom'] { color: red; --theme-fg-default: #fff; }",
			wantErr: "only custom properties are allowed",
		},
		{
			name:    "resource function",
			css:     ":root[data-theme='custom'] { --theme-bg-default: url(https://example.com/x); }",
			wantErr: "forbidden function",
		},
		{
			name:    "escaped resource function",
			css:     `:root[data-theme='custom'] { --theme-bg-default: u\72l(https://example.com/x); }`,
			wantErr: "forbidden function",
		},
		{
			name:    "escaped important",
			css:     `:root[data-theme='custom'] { --theme-bg-default: #fff !\69mportant; }`,
			wantErr: "cannot use !important",
		},
		{
			name:    "unmatched parenthesis",
			css:     `:root[data-theme='custom'] { --theme-fg-default: #fff); }`,
			wantErr: "unmatched",
		},
		{
			name:    "unmatched bracket",
			css:     `:root[data-theme='custom'] { --theme-fg-default: #fff]; }`,
			wantErr: "unmatched",
		},
		{
			name:    "crossed blocks",
			css:     `:root[data-theme='custom'] { --theme-fg-default: calc([1px)); }`,
			wantErr: "unmatched",
		},
		{
			name:    "open bracket",
			css:     `:root[data-theme='custom'] { --theme-fg-default: [#fff; }`,
			wantErr: "unclosed",
		},
		{
			name: "quoted text and escapes",
			css:  `:root[data-theme='custom'] { --theme-font-family: "Data: Sans"; --theme-label: "Data\3a  Sans"; }`,
			want: []string{`"Data: Sans"`, `"Data\3a  Sans"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			css, err := buildActiveCSS("custom", []byte(tt.css), nil)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("buildActiveCSS() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildActiveCSS() error = %v", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(string(css), want) {
					t.Fatalf("buildActiveCSS() output = %s, want containing %q", css, want)
				}
			}
		})
	}
}

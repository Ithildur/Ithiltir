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

func TestBuildActiveCSSRejectsUnexpectedSelector(t *testing.T) {
	_, err := buildActiveCSS("custom", []byte(".bad { --theme-fg-default: #fff; }"), nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported selector") {
		t.Fatalf("expected unsupported selector error, got %v", err)
	}
}

func TestBuildActiveCSSRejectsStandardProperty(t *testing.T) {
	_, err := buildActiveCSS(
		"custom",
		[]byte(":root[data-theme='custom'] { color: red; --theme-fg-default: #fff; }"),
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "only custom properties are allowed") {
		t.Fatalf("expected standard property rejection, got %v", err)
	}
}

func TestBuildActiveCSSRejectsForbiddenValue(t *testing.T) {
	_, err := buildActiveCSS(
		"custom",
		[]byte(":root[data-theme='custom'] { --theme-bg-default: url(https://example.com/x); }"),
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "forbidden token") {
		t.Fatalf("expected forbidden value rejection, got %v", err)
	}
}

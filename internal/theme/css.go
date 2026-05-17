package theme

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var customPropertyPattern = regexp.MustCompile(`^--[a-z0-9][a-z0-9-]*$`)

var forbiddenValueTokens = []string{
	"url(",
	"expression(",
	"javascript:",
	"@import",
	"@layer",
	"@media",
	"</style",
	"<style",
}

type themeCSS struct {
	Light []cssDecl
	Dark  []cssDecl
}

type cssDecl struct {
	Name  string
	Value string
}

func buildActiveCSS(id string, tokens, recipes []byte) ([]byte, error) {
	tokenCSS, err := parseThemeCSS(id, "tokens.css", tokens)
	if err != nil {
		return nil, err
	}
	if len(tokenCSS.Light) == 0 && len(tokenCSS.Dark) == 0 {
		return nil, errors.New("tokens.css must define at least one custom property")
	}

	recipeCSS, err := parseThemeCSS(id, "recipes.css", recipes)
	if err != nil {
		return nil, err
	}

	merged := themeCSS{
		Light: append(append([]cssDecl(nil), tokenCSS.Light...), recipeCSS.Light...),
		Dark:  append(append([]cssDecl(nil), tokenCSS.Dark...), recipeCSS.Dark...),
	}
	return renderThemeCSS(merged), nil
}

func parseThemeCSS(id, source string, raw []byte) (themeCSS, error) {
	cleaned, err := stripCSSComments(string(raw))
	if err != nil {
		return themeCSS{}, fmt.Errorf("%s: %w", source, err)
	}
	if strings.TrimSpace(cleaned) == "" {
		return themeCSS{}, nil
	}

	var out themeCSS
	for i := 0; i < len(cleaned); {
		i = skipWhitespace(cleaned, i)
		if i >= len(cleaned) {
			break
		}

		open := strings.IndexByte(cleaned[i:], '{')
		if open < 0 {
			return themeCSS{}, fmt.Errorf("%s: expected selector block", source)
		}
		open += i

		selector := strings.TrimSpace(cleaned[i:open])
		if selector == "" {
			return themeCSS{}, fmt.Errorf("%s: selector cannot be empty", source)
		}

		closeIdx, err := findBlockEnd(cleaned, open+1)
		if err != nil {
			return themeCSS{}, fmt.Errorf("%s: %w", source, err)
		}

		mode, err := classifySelectorList(id, selector)
		if err != nil {
			return themeCSS{}, fmt.Errorf("%s: %w", source, err)
		}

		decls, err := parseDeclarations(source, cleaned[open+1:closeIdx])
		if err != nil {
			return themeCSS{}, err
		}

		if mode == "dark" {
			out.Dark = append(out.Dark, decls...)
		} else {
			out.Light = append(out.Light, decls...)
		}

		i = closeIdx + 1
	}
	return out, nil
}

func stripCSSComments(input string) (string, error) {
	var buf strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(input); i++ {
		ch := input[i]

		switch {
		case escaped:
			buf.WriteByte(ch)
			escaped = false
			continue
		case ch == '\\':
			buf.WriteByte(ch)
			escaped = true
			continue
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
			buf.WriteByte(ch)
			continue
		case ch == '"' && !inSingle:
			inDouble = !inDouble
			buf.WriteByte(ch)
			continue
		}

		if !inSingle && !inDouble && ch == '/' && i+1 < len(input) && input[i+1] == '*' {
			end := strings.Index(input[i+2:], "*/")
			if end < 0 {
				return "", errors.New("unterminated comment")
			}
			i += end + 3
			continue
		}

		buf.WriteByte(ch)
	}

	return buf.String(), nil
}

func findBlockEnd(input string, start int) (int, error) {
	inSingle := false
	inDouble := false
	escaped := false
	depth := 0

	for i := start; i < len(input); i++ {
		ch := input[i]

		switch {
		case escaped:
			escaped = false
			continue
		case ch == '\\':
			escaped = true
			continue
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
			continue
		case ch == '"' && !inSingle:
			inDouble = !inDouble
			continue
		}

		if inSingle || inDouble {
			continue
		}

		switch ch {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '{':
			return -1, errors.New("nested blocks are not allowed in theme css")
		case '}':
			if depth == 0 {
				return i, nil
			}
		}
	}

	return -1, errors.New("unterminated block")
}

func classifySelectorList(id, selector string) (string, error) {
	lightAllowed := map[string]struct{}{
		":root": {},
		fmt.Sprintf(":root[data-theme='%s']", id): {},
		fmt.Sprintf(`:root[data-theme="%s"]`, id): {},
	}
	darkAllowed := map[string]struct{}{
		":root.dark": {},
		fmt.Sprintf(":root.dark[data-theme='%s']", id): {},
		fmt.Sprintf(`:root.dark[data-theme="%s"]`, id): {},
	}

	mode := ""
	for _, rawPart := range strings.Split(selector, ",") {
		part := normalizeSelector(rawPart)
		switch {
		case part == "":
			return "", errors.New("selector cannot be empty")
		case isAllowedSelector(part, lightAllowed):
			if mode == "" {
				mode = "light"
			}
			if mode != "light" {
				return "", fmt.Errorf("selector list cannot mix light and dark modes: %s", selector)
			}
		case isAllowedSelector(part, darkAllowed):
			if mode == "" {
				mode = "dark"
			}
			if mode != "dark" {
				return "", fmt.Errorf("selector list cannot mix light and dark modes: %s", selector)
			}
		default:
			return "", fmt.Errorf("unsupported selector %q", part)
		}
	}

	if mode == "" {
		return "", fmt.Errorf("unsupported selector list %q", selector)
	}
	return mode, nil
}

func normalizeSelector(input string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(input)), " ")
}

func isAllowedSelector(selector string, allowed map[string]struct{}) bool {
	_, ok := allowed[selector]
	return ok
}

func parseDeclarations(source, body string) ([]cssDecl, error) {
	var decls []cssDecl
	for i := 0; i < len(body); {
		i = skipDeclarationWhitespace(body, i)
		if i >= len(body) {
			break
		}
		if body[i] == '@' {
			return nil, fmt.Errorf("%s: at-rules are not allowed in theme css", source)
		}

		nameStart := i
		for i < len(body) && body[i] != ':' {
			if body[i] == '{' || body[i] == '}' {
				return nil, fmt.Errorf("%s: invalid declaration syntax", source)
			}
			i++
		}
		if i >= len(body) {
			return nil, fmt.Errorf("%s: declaration is missing ':'", source)
		}

		name := strings.TrimSpace(body[nameStart:i])
		if !customPropertyPattern.MatchString(name) {
			if strings.HasPrefix(name, "--") {
				return nil, fmt.Errorf("%s: unsupported custom property %q", source, name)
			}
			return nil, fmt.Errorf("%s: only custom properties are allowed, got %q", source, name)
		}

		i++
		valueStart := i
		inSingle := false
		inDouble := false
		escaped := false
		depth := 0

		for i < len(body) {
			ch := body[i]
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '\'' && !inDouble:
				inSingle = !inSingle
			case ch == '"' && !inSingle:
				inDouble = !inDouble
			case !inSingle && !inDouble && ch == '(':
				depth++
			case !inSingle && !inDouble && ch == ')':
				if depth > 0 {
					depth--
				}
			case !inSingle && !inDouble && depth == 0 && ch == ';':
				goto parsedValue
			case !inSingle && !inDouble && (ch == '{' || ch == '}'):
				return nil, fmt.Errorf("%s: nested blocks are not allowed in theme css", source)
			}
			i++
		}

	parsedValue:
		value := strings.TrimSpace(body[valueStart:i])
		if value == "" {
			return nil, fmt.Errorf("%s: custom property %q cannot be empty", source, name)
		}
		if strings.Contains(strings.ToLower(value), "!important") {
			return nil, fmt.Errorf("%s: custom property %q cannot use !important", source, name)
		}
		lowerValue := strings.ToLower(value)
		for _, token := range forbiddenValueTokens {
			if strings.Contains(lowerValue, token) {
				return nil, fmt.Errorf("%s: custom property %q uses forbidden token %q", source, name, token)
			}
		}

		decls = append(decls, cssDecl{Name: name, Value: value})
		if i < len(body) && body[i] == ';' {
			i++
		}
	}

	return decls, nil
}

func skipWhitespace(input string, i int) int {
	for i < len(input) {
		switch input[i] {
		case ' ', '\n', '\r', '\t', '\f':
			i++
		default:
			return i
		}
	}
	return i
}

func skipDeclarationWhitespace(input string, i int) int {
	for i < len(input) {
		switch input[i] {
		case ' ', '\n', '\r', '\t', '\f', ';':
			i++
		default:
			return i
		}
	}
	return i
}

func renderThemeCSS(css themeCSS) []byte {
	var buf bytes.Buffer
	buf.WriteString("/* active theme */\n")
	writeThemeBlock(&buf, ":root", css.Light)
	if len(css.Dark) > 0 {
		buf.WriteString("\n")
		writeThemeBlock(&buf, ":root.dark", css.Dark)
	}
	return buf.Bytes()
}

func writeThemeBlock(buf *bytes.Buffer, selector string, decls []cssDecl) {
	if len(decls) == 0 {
		return
	}
	buf.WriteString(selector)
	buf.WriteString(" {\n")
	for _, decl := range decls {
		buf.WriteString("  ")
		buf.WriteString(decl.Name)
		buf.WriteString(": ")
		buf.WriteString(decl.Value)
		buf.WriteString(";\n")
	}
	buf.WriteString("}\n")
}

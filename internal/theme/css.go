package theme

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxThemeCSSBytes   = 1 << 20
	maxThemeDecls      = 1024
	maxThemeValueRunes = 4096
)

var customPropertyPattern = regexp.MustCompile(`^--[a-z0-9][a-z0-9-]*$`)

var forbiddenThemeFunctions = map[string]struct{}{
	"expression":        {},
	"image-set":         {},
	"-webkit-image-set": {},
	"src":               {},
	"url":               {},
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
	if len(raw) > maxThemeCSSBytes {
		return themeCSS{}, fmt.Errorf("%s exceeds 1 MiB", source)
	}
	if !utf8.Valid(raw) {
		return themeCSS{}, fmt.Errorf("%s must be valid UTF-8", source)
	}
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
		if len(out.Light)+len(out.Dark) > maxThemeDecls {
			return themeCSS{}, fmt.Errorf("%s contains too many declarations", source)
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
		if len(name) > 128 || !customPropertyPattern.MatchString(name) {
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
		blocks := make([]byte, 0, 4)

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
			case !inSingle && !inDouble && (ch == '(' || ch == '['):
				blocks = append(blocks, ch)
			case !inSingle && !inDouble && (ch == ')' || ch == ']'):
				if len(blocks) == 0 || !closesCSSValueBlock(blocks[len(blocks)-1], ch) {
					return nil, fmt.Errorf("%s: custom property %q value has unmatched %q", source, name, ch)
				}
				blocks = blocks[:len(blocks)-1]
			case !inSingle && !inDouble && len(blocks) == 0 && ch == ';':
				goto parsedValue
			case !inSingle && !inDouble && (ch == '{' || ch == '}'):
				return nil, fmt.Errorf("%s: nested blocks are not allowed in theme css", source)
			}
			i++
		}
		if len(blocks) != 0 {
			return nil, fmt.Errorf("%s: custom property %q value has unclosed %q", source, name, blocks[len(blocks)-1])
		}

	parsedValue:
		value := strings.TrimSpace(body[valueStart:i])
		if value == "" {
			return nil, fmt.Errorf("%s: custom property %q cannot be empty", source, name)
		}
		if utf8.RuneCountInString(value) > maxThemeValueRunes {
			return nil, fmt.Errorf("%s: custom property %q value is too long", source, name)
		}
		if err := validateThemeValue(value); err != nil {
			return nil, fmt.Errorf("%s: custom property %q: %w", source, name, err)
		}

		decls = append(decls, cssDecl{Name: name, Value: value})
		if i < len(body) && body[i] == ';' {
			i++
		}
	}

	return decls, nil
}

func closesCSSValueBlock(open, close byte) bool {
	return open == '(' && close == ')' || open == '[' && close == ']'
}

func validateThemeValue(value string) error {
	for _, r := range value {
		if unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r' {
			return errors.New("value contains a forbidden control character")
		}
	}

	var quote byte
	for i := 0; i < len(value); {
		ch := value[i]
		if quote != 0 {
			switch ch {
			case '\\':
				next, err := skipCSSStringEscape(value, i)
				if err != nil {
					return err
				}
				i = next
				continue
			case quote:
				quote = 0
			case '\n', '\r', '\f':
				return errors.New("value contains an unescaped newline in a string")
			}
			i++
			continue
		}

		if ch == '\'' || ch == '"' {
			quote = ch
			i++
			continue
		}
		if ch == '!' {
			start := skipCSSValueWhitespace(value, i+1)
			ident, _, err := readCSSIdentifier(value, start)
			if err != nil {
				return err
			}
			if strings.EqualFold(ident, "important") {
				return errors.New("value cannot use !important")
			}
			i++
			continue
		}
		if !startsCSSIdentifier(value, i) {
			i++
			continue
		}

		ident, end, err := readCSSIdentifier(value, i)
		if err != nil {
			return err
		}
		if end < len(value) && value[end] == '(' {
			name := strings.ToLower(ident)
			if _, forbidden := forbiddenThemeFunctions[name]; forbidden {
				return fmt.Errorf("value uses forbidden function %q", name)
			}
		}
		i = end
	}
	if quote != 0 {
		return errors.New("value contains an unterminated string")
	}
	return nil
}

func startsCSSIdentifier(value string, i int) bool {
	if i >= len(value) {
		return false
	}
	ch := value[i]
	return ch == '\\' || ch == '-' || ch == '_' || ch >= 0x80 ||
		ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9'
}

func readCSSIdentifier(value string, start int) (string, int, error) {
	if !startsCSSIdentifier(value, start) {
		return "", start, nil
	}
	var out strings.Builder
	for i := start; i < len(value); {
		ch := value[i]
		switch {
		case ch == '\\':
			r, next, err := readCSSEscape(value, i)
			if err != nil {
				return "", start, err
			}
			out.WriteRune(r)
			i = next
		case ch == '-' || ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9':
			out.WriteByte(ch)
			i++
		case ch >= 0x80:
			r, size := utf8.DecodeRuneInString(value[i:])
			out.WriteRune(r)
			i += size
		default:
			return out.String(), i, nil
		}
	}
	return out.String(), len(value), nil
}

func readCSSEscape(value string, start int) (rune, int, error) {
	i := start + 1
	if i >= len(value) {
		return 0, start, errors.New("value contains an incomplete CSS escape")
	}
	if value[i] == '\n' || value[i] == '\r' || value[i] == '\f' {
		return 0, start, errors.New("value contains an invalid CSS identifier escape")
	}
	if !isCSSHex(value[i]) {
		r, size := utf8.DecodeRuneInString(value[i:])
		return r, i + size, nil
	}

	end := i
	for end < len(value) && end-i < 6 && isCSSHex(value[end]) {
		end++
	}
	decoded, err := strconv.ParseUint(value[i:end], 16, 32)
	if err != nil {
		return 0, start, fmt.Errorf("decode CSS escape: %w", err)
	}
	r := rune(decoded)
	if r == 0 || r > utf8.MaxRune || r >= 0xD800 && r <= 0xDFFF {
		r = utf8.RuneError
	}
	if end < len(value) && isCSSValueWhitespace(value[end]) {
		if value[end] == '\r' && end+1 < len(value) && value[end+1] == '\n' {
			end++
		}
		end++
	}
	return r, end, nil
}

func skipCSSStringEscape(value string, start int) (int, error) {
	if start+1 >= len(value) {
		return start, errors.New("value contains an incomplete CSS string escape")
	}
	next := start + 1
	if value[next] == '\r' && next+1 < len(value) && value[next+1] == '\n' {
		return next + 2, nil
	}
	if value[next] == '\n' || value[next] == '\r' || value[next] == '\f' {
		return next + 1, nil
	}
	_, end, err := readCSSEscape(value, start)
	return end, err
}

func skipCSSValueWhitespace(value string, i int) int {
	for i < len(value) && isCSSValueWhitespace(value[i]) {
		i++
	}
	return i
}

func isCSSValueWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '\f'
}

func isCSSHex(ch byte) bool {
	return ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F'
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

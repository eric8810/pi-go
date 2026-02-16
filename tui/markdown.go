package tui

import (
	"strings"
)

// Markdown is a component that renders markdown-formatted text to styled
// terminal output. It supports headers, bold, italic, code, code blocks,
// lists, links, horizontal rules, block quotes, and strikethrough.
type Markdown struct {
	BaseComponent
	source string
}

// NewMarkdown creates a new Markdown component with the given markdown source.
func NewMarkdown(source string) *Markdown {
	return &Markdown{
		BaseComponent: BaseComponent{dirty: true},
		source:        source,
	}
}

// SetSource updates the markdown source text.
func (m *Markdown) SetSource(source string) {
	m.source = source
	m.MarkDirty()
}

// Source returns the current markdown source text.
func (m *Markdown) Source() string {
	return m.source
}

// Render parses the markdown source and produces styled terminal output lines.
func (m *Markdown) Render(width int) []string {
	if cached := m.GetCached(width); cached != nil {
		return cached
	}

	lines := m.renderMarkdown(m.source, width)
	m.SetCached(width, lines)
	return lines
}

// renderMarkdown is the main rendering function for markdown content.
func (m *Markdown) renderMarkdown(source string, width int) []string {
	if width <= 0 {
		return []string{""}
	}

	rawLines := strings.Split(source, "\n")
	var result []string
	inCodeBlock := false
	codeBlockLang := ""
	var codeLines []string

	for i := 0; i < len(rawLines); i++ {
		line := rawLines[i]

		// Code block toggle.
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				// End code block - render accumulated lines.
				result = append(result, renderCodeBlock(codeLines, codeBlockLang, width)...)
				inCodeBlock = false
				codeBlockLang = ""
				codeLines = nil
				continue
			}
			// Start code block.
			inCodeBlock = true
			codeBlockLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			codeLines = nil
			continue
		}

		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// Horizontal rule.
		if isHorizontalRule(trimmed) {
			hrLine := DimOn + strings.Repeat("─", width) + DimOff
			result = append(result, hrLine)
			continue
		}

		// Headers.
		if strings.HasPrefix(trimmed, "#") {
			headerLines := renderHeader(trimmed, width)
			result = append(result, headerLines...)
			continue
		}

		// Block quote.
		if strings.HasPrefix(trimmed, "> ") || trimmed == ">" {
			quoteContent := strings.TrimPrefix(trimmed, "> ")
			if trimmed == ">" {
				quoteContent = ""
			}
			quoteLines := renderBlockQuote(quoteContent, width)
			result = append(result, quoteLines...)
			continue
		}

		// Unordered list.
		if isUnorderedListItem(trimmed) {
			listLines := renderUnorderedListItem(line, width)
			result = append(result, listLines...)
			continue
		}

		// Ordered list.
		if prefix, content, ok := isOrderedListItem(trimmed); ok {
			listLines := renderOrderedListItem(prefix, content, line, width)
			result = append(result, listLines...)
			continue
		}

		// Empty line.
		if trimmed == "" {
			result = append(result, "")
			continue
		}

		// Regular paragraph - apply inline formatting.
		formatted := applyInlineFormatting(line)
		wrapped := WrapText(formatted, width)
		result = append(result, wrapped...)
	}

	// If code block was never closed, render what we have.
	if inCodeBlock && len(codeLines) > 0 {
		result = append(result, renderCodeBlock(codeLines, codeBlockLang, width)...)
	}

	return result
}

// renderHeader renders a markdown header with appropriate styling.
func renderHeader(line string, width int) []string {
	level := 0
	for _, c := range line {
		if c == '#' {
			level++
		} else {
			break
		}
	}
	if level > 6 {
		level = 6
	}

	content := strings.TrimSpace(strings.TrimLeft(line, "#"))

	var styled string
	switch level {
	case 1:
		styled = BoldOn + FgBrightWhite + content + Reset
	case 2:
		styled = BoldOn + FgBrightCyan + content + Reset
	case 3:
		styled = BoldOn + FgCyan + content + Reset
	case 4:
		styled = BoldOn + content + Reset
	case 5:
		styled = DimOn + BoldOn + content + Reset
	default:
		styled = DimOn + content + Reset
	}

	wrapped := WrapText(styled, width)
	return wrapped
}

// renderCodeBlock renders a fenced code block with syntax highlighting.
func renderCodeBlock(lines []string, lang string, width int) []string {
	var result []string

	bgColor := "\033[48;5;236m" // Dark gray background.
	fgColor := "\033[38;5;252m" // Light gray text.

	// Top border.
	topLine := bgColor + fgColor
	if lang != "" {
		topLine += " " + DimOn + lang + DimOff + " "
		remaining := width - VisibleWidth(lang) - 2
		if remaining > 0 {
			topLine += strings.Repeat(" ", remaining)
		}
	} else {
		topLine += strings.Repeat(" ", width)
	}
	topLine += Reset
	result = append(result, topLine)

	// Code lines.
	for _, cl := range lines {
		codeLine := bgColor + fgColor + " "
		highlighted := highlightSyntax(cl, lang)
		codeLine += highlighted

		// Pad to width.
		visW := VisibleWidth(" " + cl)
		if visW < width {
			codeLine += strings.Repeat(" ", width-visW)
		}
		codeLine += Reset
		result = append(result, codeLine)
	}

	// Bottom border (empty line with background).
	bottomLine := bgColor + strings.Repeat(" ", width) + Reset
	result = append(result, bottomLine)

	return result
}

// highlightSyntax applies basic syntax highlighting based on language.
// This is a simplified highlighter that handles common patterns.
func highlightSyntax(line string, lang string) string {
	if lang == "" {
		return line
	}

	// Simple keyword-based highlighting.
	switch lang {
	case "go", "golang":
		return highlightGo(line)
	case "js", "javascript", "ts", "typescript":
		return highlightJS(line)
	case "py", "python":
		return highlightPython(line)
	case "sh", "bash", "shell", "zsh":
		return highlightShell(line)
	default:
		return highlightGeneric(line)
	}
}

// highlightGo applies basic Go syntax highlighting.
func highlightGo(line string) string {
	keywords := []string{
		"package", "import", "func", "return", "if", "else", "for",
		"range", "switch", "case", "default", "var", "const", "type",
		"struct", "interface", "map", "chan", "go", "defer", "select",
		"break", "continue", "goto", "fallthrough", "nil", "true", "false",
	}
	return highlightWithKeywords(line, keywords)
}

// highlightJS applies basic JavaScript/TypeScript syntax highlighting.
func highlightJS(line string) string {
	keywords := []string{
		"function", "const", "let", "var", "return", "if", "else",
		"for", "while", "switch", "case", "default", "class", "extends",
		"import", "export", "from", "async", "await", "new", "this",
		"try", "catch", "throw", "finally", "null", "undefined",
		"true", "false", "typeof", "instanceof",
	}
	return highlightWithKeywords(line, keywords)
}

// highlightPython applies basic Python syntax highlighting.
func highlightPython(line string) string {
	keywords := []string{
		"def", "class", "return", "if", "elif", "else", "for", "while",
		"import", "from", "as", "try", "except", "finally", "raise",
		"with", "yield", "lambda", "pass", "break", "continue",
		"True", "False", "None", "and", "or", "not", "in", "is",
	}
	return highlightWithKeywords(line, keywords)
}

// highlightShell applies basic shell syntax highlighting.
func highlightShell(line string) string {
	keywords := []string{
		"if", "then", "else", "elif", "fi", "for", "do", "done",
		"while", "case", "esac", "function", "return", "exit",
		"echo", "export", "local", "readonly", "set", "unset",
	}
	return highlightWithKeywords(line, keywords)
}

// highlightGeneric applies minimal highlighting for unknown languages.
func highlightGeneric(line string) string {
	return highlightStringsAndComments(line)
}

// highlightWithKeywords applies keyword highlighting and string/comment detection.
func highlightWithKeywords(line string, keywords []string) string {
	// First handle strings and comments which take priority.
	line = highlightStringsAndComments(line)

	keywordColor := "\033[38;5;204m" // Pink/magenta for keywords.

	for _, kw := range keywords {
		// Only highlight whole words by checking boundaries.
		line = highlightKeyword(line, kw, keywordColor)
	}

	return line
}

// highlightKeyword highlights whole-word occurrences of a keyword.
func highlightKeyword(line, keyword, color string) string {
	// Work on the stripped version to find positions, but we need to handle
	// ANSI codes. Simple approach: just do string replacement for exact word
	// boundaries, being careful about ANSI codes.
	stripped := StripAnsi(line)
	if !strings.Contains(stripped, keyword) {
		return line
	}

	// Simple whole-word replacement that avoids breaking ANSI sequences.
	var result strings.Builder
	i := 0
	runes := []rune(line)
	kwRunes := []rune(keyword)
	kwLen := len(kwRunes)

	for i < len(runes) {
		// Skip ANSI escape sequences.
		if runes[i] == 0x1b {
			j := i
			for j < len(runes) && !isAnsiTerminator(runes[j]) && j > i {
				j++
			}
			if j < len(runes) {
				j++ // Include the terminator.
			}
			for k := i; k < j; k++ {
				result.WriteRune(runes[k])
			}
			i = j
			continue
		}

		// Check for keyword match.
		if i+kwLen <= len(runes) && matchesWord(runes, i, kwRunes) {
			// Check word boundaries.
			leftOk := i == 0 || !isWordChar(runes[i-1])
			rightOk := i+kwLen >= len(runes) || !isWordChar(runes[i+kwLen])
			if leftOk && rightOk {
				result.WriteString(color)
				for k := 0; k < kwLen; k++ {
					result.WriteRune(runes[i+k])
				}
				result.WriteString(Reset + "\033[38;5;252m")
				i += kwLen
				continue
			}
		}

		result.WriteRune(runes[i])
		i++
	}

	return result.String()
}

// highlightStringsAndComments highlights string literals and comments.
func highlightStringsAndComments(line string) string {
	stringColor := "\033[38;5;143m"  // Yellow-green for strings.
	commentColor := "\033[38;5;240m" // Gray for comments.
	numberColor := "\033[38;5;141m"  // Purple for numbers.

	var result strings.Builder
	runes := []rune(line)
	i := 0

	for i < len(runes) {
		// Line comments.
		if i+1 < len(runes) && runes[i] == '/' && runes[i+1] == '/' {
			result.WriteString(commentColor)
			for i < len(runes) {
				result.WriteRune(runes[i])
				i++
			}
			result.WriteString(Reset + "\033[38;5;252m")
			break
		}

		// Hash comments.
		if runes[i] == '#' {
			result.WriteString(commentColor)
			for i < len(runes) {
				result.WriteRune(runes[i])
				i++
			}
			result.WriteString(Reset + "\033[38;5;252m")
			break
		}

		// String literals.
		if runes[i] == '"' || runes[i] == '\'' || runes[i] == '`' {
			quote := runes[i]
			result.WriteString(stringColor)
			result.WriteRune(runes[i])
			i++
			for i < len(runes) {
				result.WriteRune(runes[i])
				if runes[i] == quote && (i == 0 || runes[i-1] != '\\') {
					i++
					break
				}
				i++
			}
			result.WriteString(Reset + "\033[38;5;252m")
			continue
		}

		// Numbers.
		if isDigit(runes[i]) && (i == 0 || !isWordChar(runes[i-1])) {
			result.WriteString(numberColor)
			for i < len(runes) && (isDigit(runes[i]) || runes[i] == '.') {
				result.WriteRune(runes[i])
				i++
			}
			result.WriteString(Reset + "\033[38;5;252m")
			continue
		}

		result.WriteRune(runes[i])
		i++
	}

	return result.String()
}

// matchesWord checks if runes starting at pos match the keyword runes.
func matchesWord(runes []rune, pos int, keyword []rune) bool {
	for k, kr := range keyword {
		if runes[pos+k] != kr {
			return false
		}
	}
	return true
}

// isAnsiTerminator returns true if the rune terminates an ANSI escape sequence.
func isAnsiTerminator(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

// isWordChar returns true if the rune is a word character (letter, digit, underscore).
func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

// isDigit returns true if the rune is a digit.
func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// renderBlockQuote renders a block quote with a colored left border.
func renderBlockQuote(content string, width int) []string {
	borderColor := FgBrightBlue
	border := borderColor + "│" + Reset + " "
	borderWidth := 2

	innerWidth := width - borderWidth
	if innerWidth < 1 {
		innerWidth = 1
	}

	formatted := applyInlineFormatting(content)
	wrapped := WrapText(formatted, innerWidth)

	result := make([]string, len(wrapped))
	for i, line := range wrapped {
		result[i] = border + line
	}

	return result
}

// isHorizontalRule returns true if the line is a markdown horizontal rule.
func isHorizontalRule(line string) bool {
	stripped := strings.TrimSpace(line)
	if len(stripped) < 3 {
		return false
	}
	allDashes := true
	allStars := true
	allUnderscores := true
	for _, c := range stripped {
		if c != '-' && c != ' ' {
			allDashes = false
		}
		if c != '*' && c != ' ' {
			allStars = false
		}
		if c != '_' && c != ' ' {
			allUnderscores = false
		}
	}
	return allDashes || allStars || allUnderscores
}

// isUnorderedListItem checks if a line starts with an unordered list marker.
func isUnorderedListItem(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) < 2 {
		return false
	}
	return (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && trimmed[1] == ' '
}

// renderUnorderedListItem renders a single unordered list item.
func renderUnorderedListItem(line string, width int) []string {
	// Determine indent level.
	indent := 0
	for _, c := range line {
		if c == ' ' {
			indent++
		} else if c == '\t' {
			indent += 4
		} else {
			break
		}
	}

	trimmed := strings.TrimLeft(line, " \t")
	content := trimmed[2:] // Skip "- " or "* " or "+ "

	bullet := "  " + strings.Repeat("  ", indent/2) + "• "
	bulletWidth := VisibleWidth(bullet)

	innerWidth := width - bulletWidth
	if innerWidth < 1 {
		innerWidth = 1
	}

	formatted := applyInlineFormatting(content)
	wrapped := WrapText(formatted, innerWidth)

	result := make([]string, len(wrapped))
	for i, wl := range wrapped {
		if i == 0 {
			result[i] = bullet + wl
		} else {
			result[i] = strings.Repeat(" ", bulletWidth) + wl
		}
	}

	return result
}

// isOrderedListItem checks if a line is an ordered list item and returns
// the prefix and content.
func isOrderedListItem(line string) (string, string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) < 3 {
		return "", "", false
	}

	i := 0
	for i < len(trimmed) && trimmed[i] >= '0' && trimmed[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(trimmed) {
		return "", "", false
	}
	if trimmed[i] != '.' || i+1 >= len(trimmed) || trimmed[i+1] != ' ' {
		return "", "", false
	}

	prefix := trimmed[:i+1]
	content := trimmed[i+2:]
	return prefix, content, true
}

// renderOrderedListItem renders a single ordered list item.
func renderOrderedListItem(prefix, content, originalLine string, width int) []string {
	indent := 0
	for _, c := range originalLine {
		if c == ' ' {
			indent++
		} else if c == '\t' {
			indent += 4
		} else {
			break
		}
	}

	bullet := "  " + strings.Repeat("  ", indent/2) + prefix + " "
	bulletWidth := VisibleWidth(bullet)

	innerWidth := width - bulletWidth
	if innerWidth < 1 {
		innerWidth = 1
	}

	formatted := applyInlineFormatting(content)
	wrapped := WrapText(formatted, innerWidth)

	result := make([]string, len(wrapped))
	for i, wl := range wrapped {
		if i == 0 {
			result[i] = bullet + wl
		} else {
			result[i] = strings.Repeat(" ", bulletWidth) + wl
		}
	}

	return result
}

// applyInlineFormatting applies inline markdown formatting (bold, italic,
// code, strikethrough, links) to a line of text.
func applyInlineFormatting(line string) string {
	runes := []rune(line)
	var result []rune
	i := 0

	for i < len(runes) {
		// Inline code: `text`
		if runes[i] == '`' {
			end := findClosing(runes, i+1, '`')
			if end > i {
				codeContent := string(runes[i+1 : end])
				styled := "\033[48;5;238m\033[38;5;210m" + codeContent + Reset
				result = append(result, []rune(styled)...)
				i = end + 1
				continue
			}
		}

		// Strikethrough: ~~text~~
		if i+1 < len(runes) && runes[i] == '~' && runes[i+1] == '~' {
			end := findDoubleClosing(runes, i+2, '~')
			if end > i {
				content := string(runes[i+2 : end])
				styled := StrikethroughOn + content + StrikethroughOff
				result = append(result, []rune(styled)...)
				i = end + 2
				continue
			}
		}

		// Bold: **text** or __text__
		if i+1 < len(runes) && ((runes[i] == '*' && runes[i+1] == '*') || (runes[i] == '_' && runes[i+1] == '_')) {
			marker := runes[i]
			end := findDoubleClosing(runes, i+2, marker)
			if end > i {
				content := string(runes[i+2 : end])
				// Recursively apply inline formatting to the content.
				formatted := applyInlineFormatting(content)
				styled := BoldOn + formatted + BoldOff
				result = append(result, []rune(styled)...)
				i = end + 2
				continue
			}
		}

		// Italic: *text* or _text_
		if (runes[i] == '*' || runes[i] == '_') && i+1 < len(runes) && runes[i+1] != runes[i] {
			marker := runes[i]
			end := findClosing(runes, i+1, marker)
			if end > i+1 {
				content := string(runes[i+1 : end])
				formatted := applyInlineFormatting(content)
				styled := ItalicOn + formatted + ItalicOff
				result = append(result, []rune(styled)...)
				i = end + 1
				continue
			}
		}

		// Links: [text](url)
		if runes[i] == '[' {
			textEnd := findClosing(runes, i+1, ']')
			if textEnd > i && textEnd+1 < len(runes) && runes[textEnd+1] == '(' {
				urlEnd := findClosing(runes, textEnd+2, ')')
				if urlEnd > textEnd+1 {
					linkText := string(runes[i+1 : textEnd])
					linkURL := string(runes[textEnd+2 : urlEnd])
					styled := UnderlineOn + linkText + UnderlineOff + DimOn + " (" + linkURL + ")" + DimOff
					result = append(result, []rune(styled)...)
					i = urlEnd + 1
					continue
				}
			}
		}

		result = append(result, runes[i])
		i++
	}

	return string(result)
}

// findClosing finds the index of the closing character starting from pos.
// Returns -1 if not found.
func findClosing(runes []rune, start int, closing rune) int {
	for i := start; i < len(runes); i++ {
		if runes[i] == closing {
			return i
		}
	}
	return -1
}

// findDoubleClosing finds the index of a double closing character (e.g., **)
// starting from pos. Returns -1 if not found.
func findDoubleClosing(runes []rune, start int, closing rune) int {
	for i := start; i+1 < len(runes); i++ {
		if runes[i] == closing && runes[i+1] == closing {
			return i
		}
	}
	return -1
}

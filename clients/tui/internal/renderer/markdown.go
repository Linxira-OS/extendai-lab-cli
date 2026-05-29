package renderer

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// RenderMarkdown renders markdown with explicit token-level terminal styling.
//
// Design goals:
// - Headings use distinct colors by level
// - Strong/emphasis/link/list tokens get their own colors
// - Lists preserve indentation and do not spill outside their block
// - Code fences remain fenced and readable
// - Tables are left to the table renderer elsewhere
func RenderMarkdown(content string, width int) string {
	if content == "" {
		return ""
	}
	if width < 20 {
		width = 20
	}

	content = preprocessMarkdown(content)
	lines := strings.Split(content, "\n")
	var out []string
	var inCode bool
	var codeBuf []string

	flushCode := func() {
		if len(codeBuf) == 0 {
			return
		}
		fenced := "```"
		out = append(out, theme.Heading4Style.Render(fenced))
		for _, ln := range codeBuf {
			out = append(out, lipgloss.NewStyle().Foreground(theme.Colors.Text).Render(ln))
		}
		out = append(out, theme.Heading4Style.Render(fenced))
		codeBuf = codeBuf[:0]
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			if inCode {
				flushCode()
				inCode = false
				continue
			}
			inCode = true
			continue
		}
		if inCode {
			codeBuf = append(codeBuf, line)
			continue
		}

		if trimmed == "" {
			out = append(out, "")
			continue
		}

		if rendered, ok := renderHeading(line); ok {
			out = append(out, rendered)
			continue
		}

		if rendered, ok := renderList(line); ok {
			out = append(out, rendered)
			continue
		}

		if rendered, ok := renderBlockquote(line); ok {
			out = append(out, rendered)
			continue
		}

		out = append(out, renderInlineMarkdown(line))
	}

	flushCode()
	return wrapLines(out, width)
}

func renderHeading(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return "", false
	}
	if level < len(trimmed) && trimmed[level] != ' ' {
		return "", false
	}
	text := strings.TrimSpace(trimmed[level:])
	switch level {
	case 1:
		return theme.Heading1Style.Render(text), true
	case 2:
		return theme.Heading2Style.Render(text), true
	case 3:
		return theme.Heading3Style.Render(text), true
	case 4:
		return theme.Heading4Style.Render(text), true
	case 5:
		return theme.Heading5Style.Render(text), true
	default:
		return theme.Heading6Style.Render(text), true
	}
}

func renderList(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	indent := len(line) - len(trimmed)
	prefix := ""
	body := ""

	ordered := regexp.MustCompile(`^\d+[\.)] `)
	unordered := regexp.MustCompile(`^[-*+] `)

	switch {
	case ordered.MatchString(trimmed):
		idx := strings.Index(trimmed, " ")
		prefix = trimmed[:idx]
		body = strings.TrimSpace(trimmed[idx:])
		prefix = lipgloss.NewStyle().Foreground(theme.Colors.Secondary).Render(prefix)
	case unordered.MatchString(trimmed):
		prefix = lipgloss.NewStyle().Foreground(theme.Colors.Secondary).Render("•")
		body = strings.TrimSpace(trimmed[2:])
	default:
		return "", false
	}

	pad := strings.Repeat(" ", indent)
	return pad + prefix + " " + renderInlineMarkdown(body), true
}

func renderBlockquote(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if !strings.HasPrefix(trimmed, ">") {
		return "", false
	}
	body := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
	bar := lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render("│")
	text := lipgloss.NewStyle().Foreground(theme.Colors.TextSoft).Italic(true).Render(renderInlineMarkdown(body))
	return bar + " " + text, true
}

func renderInlineMarkdown(line string) string {
	// Strong first so it doesn't get partially consumed by italic.
	line = replaceDelimited(line, "**", theme.Heading2Style)
	line = replaceDelimited(line, "__", theme.Heading2Style)

	// Italic/emphasis gets a different color from strong.
	line = replaceDelimited(line, "*", lipgloss.NewStyle().Foreground(theme.Colors.Accent).Italic(true))
	line = replaceDelimited(line, "_", lipgloss.NewStyle().Foreground(theme.Colors.Accent).Italic(true))

	// Inline links: [text](url) -> colored text + dim URL.
	linkRe := regexp.MustCompile(`\[(.+?)\]\((.+?)\)`)
	line = linkRe.ReplaceAllStringFunc(line, func(s string) string {
		m := linkRe.FindStringSubmatch(s)
		if len(m) != 3 {
			return s
		}
		text := lipgloss.NewStyle().Foreground(theme.Colors.Secondary).Render(m[1])
		url := lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render("(" + m[2] + ")")
		return text + url
	})

	return line
}

func replaceDelimited(line, delim string, style lipgloss.Style) string {
	if delim == "" {
		return line
	}
	marker := string([]rune(delim)[0])
	pattern := regexp.MustCompile(regexp.QuoteMeta(delim) + "([^" + regexp.QuoteMeta(marker) + "]+?)" + regexp.QuoteMeta(delim))
	return pattern.ReplaceAllStringFunc(line, func(s string) string {
		m := pattern.FindStringSubmatch(s)
		if len(m) != 2 {
			return s
		}
		return style.Render(m[1])
	})
}

func preprocessMarkdown(s string) string {
	s = strings.ReplaceAll(s, "\t", "    ")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

func wrapLines(lines []string, width int) string {
	var out []string
	for _, line := range lines {
		if line == "" {
			out = append(out, "")
			continue
		}
		// Use lipgloss.Width to correctly handle ANSI escape codes
		lineWidth := lipgloss.Width(line)
		if lineWidth <= width {
			out = append(out, line)
			continue
		}
		out = append(out, hardWrap(line, width)...)
	}
	return strings.Join(out, "\n")
}

func hardWrap(s string, width int) []string {
	if width < 1 {
		return []string{s}
	}

	// For strings with ANSI codes, we need to be more careful
	// First, try to wrap at word boundaries
	var out []string
	remaining := s

	for lipgloss.Width(remaining) > width {
		// Find a good break point
		cut := findBreakPoint(remaining, width)
		if cut <= 0 {
			// Can't find a good break point, force break
			cut = width
		}

		// Extract the line
		line := remaining[:cut]
		out = append(out, strings.TrimRight(line, " "))
		remaining = strings.TrimLeft(remaining[cut:], " ")
	}

	if remaining != "" {
		out = append(out, remaining)
	}

	return out
}

// findBreakPoint finds a good point to break a line, preferring word boundaries.
func findBreakPoint(s string, width int) int {
	// Walk through the string, tracking visible width
	visibleWidth := 0
	lastSpace := -1
	lastSpaceWidth := 0

	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])

		// Check for ANSI escape sequence
		if r == '\033' {
			// Skip the entire escape sequence
			end := i + 1
			for end < len(s) && s[end] != 'm' {
				end++
			}
			if end < len(s) {
				end++ // Include the 'm'
			}
			i = end
			continue
		}

		// Track visible width
		if r == ' ' {
			lastSpace = i
			lastSpaceWidth = visibleWidth
		}

		visibleWidth++
		if visibleWidth > width {
			// We've exceeded the width
			if lastSpace > 0 && lastSpaceWidth > width/2 {
				// Break at the last space if it's past halfway
				return lastSpace
			}
			// Otherwise break at current position
			return i
		}

		i += size
	}

	return len(s)
}

// runeIndexAtWidth returns the byte offset into s after width runes.
func runeIndexAtWidth(s string, width int) int {
	if width <= 0 {
		return 0
	}
	count := 0
	for i := range s {
		if count == width {
			return i
		}
		count++
	}
	return len(s)
}

// lastSpaceBefore finds the byte offset of the last space before byte position cut,
// walking backwards by rune boundaries to avoid splitting multi-byte characters.
// Returns -1 if no space is found.
func lastSpaceBefore(s string, cut int) int {
	if cut <= 0 || cut > len(s) {
		return -1
	}
	prefix := s[:cut]
	pos := cut
	for pos > 0 {
		r, size := utf8.DecodeLastRuneInString(prefix[:pos])
		if r == ' ' {
			return pos - size
		}
		pos -= size
		if pos <= 0 {
			break
		}
	}
	return -1
}

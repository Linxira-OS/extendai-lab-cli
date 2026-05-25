package protocol

// ─── RenderCommand ──────────────────────────────────────────

// RenderCommand is the top-level message from TS to Go TUI.
type RenderCommand struct {
	Type       string          `json:"type"`                // "render"
	Seq        int             `json:"seq"`
	Components []Component     `json:"components"`
	Layout     *LayoutSpec     `json:"layout,omitempty"`
	Theme      *ThemeOverrides `json:"theme,omitempty"`
}

// Acknowledgement sent from Go TUI back to TS.
type Ack struct {
	Type string `json:"type"` // "ack"
	Seq  int    `json:"seq"`
	OK   bool   `json:"ok"`
	Err  string `json:"err,omitempty"`
}

// LayoutSpec defines how components are arranged.
type LayoutSpec struct {
	Direction   string   `json:"direction"`             // "vertical" | "horizontal"
	SplitRatios []float64 `json:"splitRatios,omitempty"`
	Focus       string   `json:"focus,omitempty"`
}

// ─── Component ──────────────────────────────────────────────

type Component struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Props    map[string]interface{} `json:"props"`
	Children []Component            `json:"children,omitempty"`
}

// ─── Component Types (constants) ───────────────────────────

const (
	CompMessageList = "message-list"
	CompMessage     = "message"
	CompInput       = "input"
	CompStatusBar   = "status-bar"
	CompHeader      = "header"
	CompMarkdown    = "markdown"
	CompTable       = "table"
	CompDiff        = "diff"
	CompTree        = "tree"
	CompProgress    = "progress"
	CompSpinner     = "spinner"
	CompToolOutput  = "tool-output"
	CompPanel       = "panel"
	CompCustom      = "custom"
)

// ─── MessageProps ───────────────────────────────────────────

type MessageProps struct {
	Role      string `json:"role"`                // "user" | "assistant" | "system" | "tool"
	Content   string `json:"content"`
	Format    string `json:"format"`              // "markdown" | "text" | "html"
	Timestamp string `json:"timestamp,omitempty"`
	Indent    int    `json:"indent,omitempty"`
}

// ─── MarkdownRenderProps ────────────────────────────────────

type MarkdownRenderProps struct {
	Content       string `json:"content"`
	EnableTables  bool   `json:"enableTables"`
	EnableTaskList bool  `json:"enableTaskList"`
	EnableSyntaxHighlight bool `json:"enableSyntaxHighlight"`
	TabWidth      int    `json:"tabWidth"`
	MaxWidth      int    `json:"maxWidth"`
}

// DefaultMarkdownProps returns sensible defaults.
func DefaultMarkdownProps(content string) MarkdownRenderProps {
	return MarkdownRenderProps{
		Content:                content,
		EnableTables:           true,
		EnableTaskList:         true,
		EnableSyntaxHighlight:  true,
		TabWidth:               4,
		MaxWidth:               80,
	}
}

// ─── TableRenderProps ───────────────────────────────────────

type TableRenderProps struct {
	Headers      []string `json:"headers"`
	Rows         [][]string `json:"rows"`
	Alignment    []string `json:"alignment"`     // "left" | "center" | "right"
	ColumnWidths []int    `json:"columnWidths,omitempty"`
	TabReplace   string   `json:"tabReplace"`    // default: 4 spaces
}

// ─── StatusBarProps ─────────────────────────────────────────

type StatusBarProps struct {
	Left   []StatusItem `json:"left"`
	Center []StatusItem `json:"center,omitempty"`
	Right  []StatusItem `json:"right,omitempty"`
}

type StatusItem struct {
	Text   string    `json:"text"`
	Style  *TextStyle `json:"style,omitempty"`
	Action string    `json:"action,omitempty"`
}

// ─── ThemeOverrides ─────────────────────────────────────────

type ThemeOverrides struct {
	Colors  *ThemeColors  `json:"colors,omitempty"`
	Spacing *ThemeSpacing `json:"spacing,omitempty"`
}

type ThemeColors struct {
	Primary   string `json:"primary,omitempty"`
	Secondary string `json:"secondary,omitempty"`
	Accent    string `json:"accent,omitempty"`
	Success   string `json:"success,omitempty"`
	Error     string `json:"error,omitempty"`
	Warning   string `json:"warning,omitempty"`
	Muted     string `json:"muted,omitempty"`
	Surface   string `json:"surface,omitempty"`
	Base      string `json:"base,omitempty"`
	Text      string `json:"text,omitempty"`
	TextDim   string `json:"textDim,omitempty"`
}

type ThemeSpacing struct {
	Padding int `json:"padding,omitempty"`
	Gap     int `json:"gap,omitempty"`
	Margin  int `json:"margin,omitempty"`
}

// ─── TextStyle ──────────────────────────────────────────────

type TextStyle struct {
	Bold          bool     `json:"bold,omitempty"`
	Italic        bool     `json:"italic,omitempty"`
	Underline     bool     `json:"underline,omitempty"`
	Strikethrough bool     `json:"strikethrough,omitempty"`
	Foreground    string   `json:"foreground,omitempty"`
	Background    string   `json:"background,omitempty"`
	ExtraClasses  []string `json:"extraClasses,omitempty"`
}

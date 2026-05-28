package model

import (
	"unicode"
	"unicode/utf8"
)

// ─── Token Estimation (CJK-aware) ──────────────────────────────
//
// Based on deepseek-reasonix countTokensBounded:
//   - Chinese/Japanese/Korean: 1 token ≈ 1.5 characters
//   - English/ASCII: 1 token ≈ 4 characters
//   - Code (mixed): 1 token ≈ 3.5 characters
//
// Reference: studying/deepseek-reasonix/src/utils/token-estimator.ts

// EstimateTokens estimates the number of tokens in a text string.
// Uses CJK-aware estimation for better accuracy with mixed-language content.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	cjkChars := 0
	otherChars := 0

	for _, r := range text {
		if isCJK(r) {
			cjkChars++
		} else {
			otherChars++
		}
	}

	// CJK: 1 token ≈ 1.5 characters (so 1 char ≈ 0.67 tokens)
	// Other: 1 token ≈ 4 characters (so 1 char ≈ 0.25 tokens)
	cjkTokens := float64(cjkChars) * 0.67
	otherTokens := float64(otherChars) * 0.25

	return int(cjkTokens + otherTokens + 0.5) // round to nearest
}

// EstimateTokensFromRunes estimates tokens from a rune count.
// Use this when you already have the rune count (e.g., from utf8.RuneCountInString).
func EstimateTokensFromRunes(runeCount, cjkCount int) int {
	cjkTokens := float64(cjkCount) * 0.67
	otherTokens := float64(runeCount-cjkCount) * 0.25
	return int(cjkTokens + otherTokens + 0.5)
}

// CountCJK counts the number of CJK characters in a string.
func CountCJK(text string) int {
	count := 0
	for _, r := range text {
		if isCJK(r) {
			count++
		}
	}
	return count
}

// isCJK returns true if the rune is a CJK (Chinese/Japanese/Korean) character.
// Covers:
//   - CJK Unified Ideographs (U+4E00 - U+9FFF)
//   - CJK Unified Ideographs Extension A (U+3400 - U+4DBF)
//   - CJK Unified Ideographs Extension B (U+20000 - U+2A6DF)
//   - CJK Compatibility Ideographs (U+F900 - U+FAFF)
//   - CJK Radicals Supplement (U+2E80 - U+2EFF)
//   - Kangxi Radicals (U+2F00 - U+2FDF)
//   - CJK Symbols and Punctuation (U+3000 - U+303F)
//   - Hiragana (U+3040 - U+309F)
//   - Katakana (U+30A0 - U+30FF)
//   - Hangul Syllables (U+AC00 - U+D7AF)
//   - Hangul Jamo (U+1100 - U+11FF)
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r) ||
		(r >= 0x3000 && r <= 0x303F) || // CJK Symbols and Punctuation
		(r >= 0x2E80 && r <= 0x2EFF) || // CJK Radicals Supplement
		(r >= 0x2F00 && r <= 0x2FDF) || // Kangxi Radicals
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility Ideographs
		(r >= 0x20000 && r <= 0x2A6DF) // CJK Unified Ideographs Extension B
}

// EstimateMessagesTokens estimates total tokens for a slice of messages.
// This is more accurate than simple char/4 because it accounts for:
//   - CJK characters (higher token density)
//   - Message overhead (role, formatting)
func EstimateMessagesTokens(messages []Message) int {
	total := 0
	for _, msg := range messages {
		// Message overhead: role + formatting ≈ 4 tokens
		total += 4
		total += EstimateTokens(msg.Content)
	}
	return total
}

// EstimateAPIHistoryTokens estimates tokens for API history messages.
func EstimateAPIHistoryTokens(messages []struct {
	Role    string
	Content string
}) int {
	total := 0
	for _, msg := range messages {
		total += 4 // overhead
		total += EstimateTokens(msg.Content)
	}
	return total
}

// TokenStats holds detailed token statistics.
type TokenStats struct {
	TotalTokens int
	CJKChars    int
	OtherChars  int
	TotalChars  int
	CJKRatio    float64 // CJK chars / total chars
}

// AnalyzeTokens provides detailed token analysis for a string.
func AnalyzeTokens(text string) TokenStats {
	cjk := CountCJK(text)
	total := utf8.RuneCountInString(text)

	ratio := 0.0
	if total > 0 {
		ratio = float64(cjk) / float64(total)
	}

	return TokenStats{
		TotalTokens: EstimateTokens(text),
		CJKChars:    cjk,
		OtherChars:  total - cjk,
		TotalChars:  total,
		CJKRatio:    ratio,
	}
}

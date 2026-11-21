package ai

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
)

// isAsciiLetter returns true if r is 'a'-'z' or 'A'-'Z'.
func isAsciiLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isHex returns true if r is a valid hexadecimal character.
func isHex(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// RepairAndCleanJSON cleans and repairs common LLM JSON defects:
// 1. Markdown code fences (```json ... ```)
// 2. Unescaped control characters (literal newlines/tabs inside strings)
// 3. LaTeX single backslashes in strings (e.g. \frac, \alpha, \sum, \beta, \times, \theta)
// 4. Trailing commas before closing braces/brackets (, } or , ])
// 5. Truncated or unclosed JSON brackets/braces using LIFO stack balance
func RepairAndCleanJSON(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "{}"
	}

	// 1. Strip markdown fences if present
	if idx := strings.Index(s, "```json"); idx != -1 {
		s = s[idx+len("```json"):]
		if end := strings.Index(s, "```"); end != -1 {
			s = s[:end]
		}
	} else if idx := strings.Index(s, "```"); idx != -1 {
		s = s[idx+len("```"):]
		if end := strings.Index(s, "```"); end != -1 {
			s = s[:end]
		}
	}
	s = strings.TrimSpace(s)

	// 2. Extract boundary from first '{' or '['
	firstBrace := strings.IndexAny(s, "{[")
	if firstBrace != -1 {
		s = s[firstBrace:]
	}

	// 3. Remove trailing commas before } or ]
	reTrailingComma := regexp.MustCompile(`,\s*([}\]])`)
	s = reTrailingComma.ReplaceAllString(s, "$1")

	// 4. State-machine pass to sanitize strings:
	// - Fix unescaped newlines/tabs inside quotes
	// - Escape single LaTeX backslashes that are NOT valid JSON escape codes
	var buf bytes.Buffer
	inString := false
	runes := []rune(s)
	n := len(runes)

	for i := 0; i < n; i++ {
		r := runes[i]

		if !inString {
			if r == '"' {
				inString = true
			}
			buf.WriteRune(r)
			continue
		}

		// --- Inside String Literal ---
		if r == '\\' {
			if i+1 < n {
				next := runes[i+1]
				isStandardJSONEscape := false

				// Check standard JSON escapes
				switch next {
				case '"', '\\', '/':
					isStandardJSONEscape = true
				case 'u':
					// Unicode escape \uXXXX must be followed by 4 hex digits
					if i+5 < n && isHex(runes[i+2]) && isHex(runes[i+3]) && isHex(runes[i+4]) && isHex(runes[i+5]) {
						isStandardJSONEscape = true
					}
				case 'b', 'f', 'n', 'r', 't':
					// If followed by another letter, e.g. \frac, \beta, \theta, \times, \rho, \nabla,
					// it's a LaTeX macro! It should NOT be treated as a JSON control escape.
					if i+2 < n && isAsciiLetter(runes[i+2]) {
						isStandardJSONEscape = false
					} else {
						isStandardJSONEscape = true
					}
				}

				if isStandardJSONEscape {
					buf.WriteRune('\\')
					buf.WriteRune(next)
					i++ // Skip next rune as it's part of the valid escape
				} else {
					// Invalid JSON escape or LaTeX macro (e.g. \frac, \sum, \alpha, \times)
					// Double the backslash so it safely unmarshals as literal backslash in Go!
					buf.WriteString(`\\`)
				}
			} else {
				buf.WriteString(`\\`)
			}
			continue
		}

		if r == '"' {
			inString = false
			buf.WriteRune(r)
			continue
		}

		// Fix unescaped control characters inside JSON strings (RFC 8259)
		switch r {
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if unicode.IsControl(r) {
				continue
			}
			buf.WriteRune(r)
		}
	}

	res := buf.String()

	// 5. If string was not closed (truncated), close it
	if inString {
		res += `"`
	}

	// 6. Stack-based LIFO balance for truncated unclosed braces/brackets
	var stack []rune
	inStr := false
	escaped := false
	for _, ch := range res {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		switch ch {
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) > 0 && stack[len(stack)-1] == ch {
				stack = stack[:len(stack)-1]
			}
		}
	}

	// Clean any trailing commas before closing
	res = reTrailingComma.ReplaceAllString(res, "$1")

	// Pop all expected closers in LIFO order
	for i := len(stack) - 1; i >= 0; i-- {
		res += string(stack[i])
	}

	return strings.TrimSpace(res)
}

// UnmarshalJSONSafe attempts standard json.Unmarshal first.
// If it fails, it applies RepairAndCleanJSON and retries unmarshaling.
func UnmarshalJSONSafe(raw []byte, target interface{}) error {
	// Fast path: valid JSON unmarshals immediately
	err := json.Unmarshal(raw, target)
	if err == nil {
		return nil
	}

	// Repair path: repair string defects (invalid escapes, literal newlines, unclosed braces)
	repaired := RepairAndCleanJSON(string(raw))
	return json.Unmarshal([]byte(repaired), target)
}

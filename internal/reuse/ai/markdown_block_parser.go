package ai

import (
	"regexp"
	"strings"

	"goProjectStructure/internal/handler/dto"
)

var (
	checklistRegex       = regexp.MustCompile(`^-\s+\[([ xX])\]\s+(.*)`)
	numberedRegex        = regexp.MustCompile(`^(\d+)\.\s+(.*)`)
	bulletRegex          = regexp.MustCompile(`^(?:[•\-\*])\s+(.*)`)
	h4to6Regex           = regexp.MustCompile(`^#{4,6}\s+`)
	dividerRegex         = regexp.MustCompile(`^(?:[-*_]){3,}\s*$`)
	calloutPrefixRegex   = regexp.MustCompile(`^(?:>\s*)+\[!(?:note|tip|warning|important|caution)\]\s*`)
	calloutHeaderEmojiRx = regexp.MustCompile(`^(?:>\s*)+(?:💡|🎯|⚠️|🧠|🔗|📐)\s*`)
	quotePrefixRegex     = regexp.MustCompile(`^(?:>\s*)+`)
	flowArrowRegex       = regexp.MustCompile(`(?:-->|==>|->)`)
)

// ParseMarkdownBlocks parses academic markdown into structured AST blocks on the Go backend.
// Offloading this heavy 20+ regex multi-pass lexical analysis from mobile Dart to Go
// achieves butter-smooth 60fps scrolling and zero UI freeze on mobile devices.
func ParseMarkdownBlocks(text string) []dto.AiBlockDto {
	cleanText := strings.TrimSpace(text)
	if cleanText == "" {
		return nil
	}

	lines := strings.Split(cleanText, "\n")
	var blocks []dto.AiBlockDto

	var (
		insideCodeBlock    bool
		codeLanguage       string
		codeBuffer         strings.Builder
		insideTable        bool
		tableBuffer        strings.Builder
		insideCallout      bool
		calloutBuffer      strings.Builder
		currentCalloutType string
		insideFormulaBlock bool
		formulaBuffer      strings.Builder
	)

	flushTable := func() {
		if insideTable {
			content := strings.TrimSpace(tableBuffer.String())
			if content != "" {
				blocks = append(blocks, dto.AiBlockDto{
					Type: "table",
					Text: content,
				})
			}
			tableBuffer.Reset()
			insideTable = false
		}
	}

	flushCallout := func() {
		if insideCallout {
			content := strings.TrimSpace(calloutBuffer.String())
			if content != "" {
				blocks = append(blocks, dto.AiBlockDto{
					Type:  "callout",
					Text:  content,
					Extra: currentCalloutType,
				})
			}
			calloutBuffer.Reset()
			insideCallout = false
			currentCalloutType = ""
		}
	}

	flushFormula := func() {
		if insideFormulaBlock {
			content := strings.TrimSpace(formulaBuffer.String())
			if content != "" {
				if isGenuineMath(content) {
					blocks = append(blocks, dto.AiBlockDto{
						Type: "formula",
						Text: content,
					})
				} else {
					blocks = append(blocks, dto.AiBlockDto{
						Type: "paragraph",
						Text: content,
					})
				}
			}
			formulaBuffer.Reset()
			insideFormulaBlock = false
		}
	}

	for i := 0; i < len(lines); i++ {
		rawLine := lines[i]
		line := strings.TrimSpace(rawLine)

		// Code / Diagram Block Boundary (```)
		if strings.HasPrefix(line, "```") {
			flushCallout()
			flushTable()
			flushFormula()

			if insideCodeBlock {
				content := strings.TrimSpace(codeBuffer.String())
				lang := strings.ToLower(strings.TrimSpace(codeLanguage))

				if lang == "derivation" || lang == "math-derivation" || lang == "steps" || lang == "proof" || lang == "step-by-step" {
					subBlocks := ParseMarkdownBlocks(content)
					blocks = append(blocks, subBlocks...)
				} else if lang == "graph" || lang == "plot" || lang == "curve" || lang == "function-graph" || lang == "coordinate-plot" {
					blocks = append(blocks, dto.AiBlockDto{
						Type:  "graph",
						Text:  content,
						Extra: "Interactive Coordinate Plot",
					})
				} else if isDiagramLanguageOrContent(lang, content) {
					extra := codeLanguage
					if extra == "" {
						extra = "diagram"
					}
					blocks = append(blocks, dto.AiBlockDto{
						Type:  "diagram",
						Text:  content,
						Extra: extra,
					})
				} else if lang == "math" || lang == "latex" || lang == "equation" || lang == "formula" || lang == "tex" {
					blocks = append(blocks, dto.AiBlockDto{
						Type:  "formula",
						Text:  content,
						Extra: "Mathematical Equation",
					})
				} else {
					blocks = append(blocks, dto.AiBlockDto{
						Type:  "code",
						Text:  content,
						Extra: codeLanguage,
					})
				}

				codeBuffer.Reset()
				insideCodeBlock = false
				codeLanguage = ""
			} else {
				insideCodeBlock = true
				codeLanguage = strings.TrimSpace(line[3:])
			}
			continue
		}

		if insideCodeBlock {
			codeBuffer.WriteString(rawLine)
			codeBuffer.WriteString("\n")
			continue
		}

		// Multi-Line Formula Block ($$ ... $$ or \[ ... \])
		if (strings.HasPrefix(line, "$$") && !strings.HasSuffix(line, "$$")) ||
			(strings.HasPrefix(line, `\[`) && !strings.HasSuffix(line, `\]`)) {
			flushCallout()
			flushTable()
			if insideFormulaBlock {
				flushFormula()
			} else {
				insideFormulaBlock = true
				startContent := ""
				if strings.HasPrefix(line, "$$") || strings.HasPrefix(line, `\[`) {
					startContent = strings.TrimSpace(line[2:])
				}
				if startContent != "" {
					formulaBuffer.WriteString(startContent)
					formulaBuffer.WriteString("\n")
				}
				continue
			}
		} else if insideFormulaBlock {
			if strings.HasSuffix(line, "$$") || strings.HasSuffix(line, `\]`) {
				endContent := strings.TrimSpace(line[:len(line)-2])
				if endContent != "" {
					formulaBuffer.WriteString(endContent)
					formulaBuffer.WriteString("\n")
				}
				flushFormula()
				continue
			} else {
				formulaBuffer.WriteString(rawLine)
				formulaBuffer.WriteString("\n")
				continue
			}
		}

		// Markdown Table Row Detection (| Col | Col |)
		if strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") && len(line) > 2 {
			flushCallout()
			flushFormula()
			insideTable = true
			tableBuffer.WriteString(line)
			tableBuffer.WriteString("\n")
			continue
		} else if insideTable {
			flushTable()
		}

		if line == "" {
			if insideCallout {
				calloutBuffer.WriteString("\n")
				continue
			}
			continue
		}

		// Callout Boxes & Exam Tips (> ... or [!NOTE])
		lowerLine := strings.ToLower(line)
		isCalloutStart := strings.HasPrefix(line, "> ") ||
			strings.HasPrefix(lowerLine, "[!note]") ||
			strings.HasPrefix(lowerLine, "[!tip]") ||
			strings.HasPrefix(lowerLine, "[!warning]") ||
			strings.HasPrefix(lowerLine, "[!important]") ||
			strings.HasPrefix(lowerLine, "[!caution]")

		if isCalloutStart {
			calloutType := "tip"
			if strings.Contains(lowerLine, "🎯") || strings.Contains(lowerLine, "exam") || strings.Contains(lowerLine, "important") || strings.Contains(lowerLine, "score") {
				calloutType = "exam"
			} else if strings.Contains(lowerLine, "⚠️") || strings.Contains(lowerLine, "warning") || strings.Contains(lowerLine, "pitfall") || strings.Contains(lowerLine, "trap") || strings.Contains(lowerLine, "caution") {
				calloutType = "warning"
			} else if strings.Contains(lowerLine, "🧠") || strings.Contains(lowerLine, "feynman") || strings.Contains(lowerLine, "hook") {
				calloutType = "feynman"
			} else if strings.Contains(lowerLine, "🔗") || strings.Contains(lowerLine, "connected") || strings.Contains(lowerLine, "link") || strings.Contains(lowerLine, "next") {
				calloutType = "connected"
			} else if strings.Contains(lowerLine, "📐") || strings.Contains(lowerLine, "derivation") || strings.Contains(lowerLine, "step") || strings.Contains(lowerLine, "proof") {
				calloutType = "derivation"
			} else if strings.Contains(lowerLine, "💡") || strings.Contains(lowerLine, "intuition") || strings.Contains(lowerLine, "analogy") || strings.Contains(lowerLine, "insight") {
				calloutType = "intuition"
			}

			cleanLine := calloutPrefixRegex.ReplaceAllString(line, "")
			cleanLine = calloutHeaderEmojiRx.ReplaceAllString(cleanLine, "")
			cleanLine = strings.TrimSpace(cleanLine)

			if insideCallout && calloutType == currentCalloutType {
				calloutBuffer.WriteString(cleanLine)
				calloutBuffer.WriteString("\n")
			} else {
				flushCallout()
				insideCallout = true
				currentCalloutType = calloutType
				calloutBuffer.WriteString(cleanLine)
				calloutBuffer.WriteString("\n")
			}
			continue
		} else if insideCallout && strings.HasPrefix(line, ">") {
			calloutBuffer.WriteString(quotePrefixRegex.ReplaceAllString(line, ""))
			calloutBuffer.WriteString("\n")
			continue
		} else {
			flushCallout()
		}

		// Comparison highlight (❌ Common Mistake / ✅ Correct Concept)
		compLine := strings.TrimLeft(line, "-*• \t")
		lowerComp := strings.ToLower(compLine)
		if strings.HasPrefix(compLine, "❌") || strings.HasPrefix(lowerComp, "mistake:") || strings.HasPrefix(lowerComp, "common mistake:") {
			flushCallout()
			flushTable()
			flushFormula()
			blocks = append(blocks, dto.AiBlockDto{
				Type:  "comparison",
				Text:  compLine,
				Extra: "mistake",
			})
			continue
		} else if strings.HasPrefix(compLine, "✅") || strings.HasPrefix(lowerComp, "correct:") || strings.HasPrefix(lowerComp, "correct concept:") {
			flushCallout()
			flushTable()
			flushFormula()
			blocks = append(blocks, dto.AiBlockDto{
				Type:  "comparison",
				Text:  compLine,
				Extra: "correct",
			})
			continue
		}

		// Standalone ASCII Art / Diagram in text (e.g. tree lines or box lines)
		if isStandaloneDiagramText(line) {
			var diagBuffer strings.Builder
			diagBuffer.WriteString(rawLine)
			diagBuffer.WriteString("\n")
			for i+1 < len(lines) && (isStandaloneDiagramText(strings.TrimSpace(lines[i+1])) || strings.HasPrefix(strings.TrimSpace(lines[i+1]), "|")) {
				i++
				diagBuffer.WriteString(lines[i])
				diagBuffer.WriteString("\n")
			}
			blocks = append(blocks, dto.AiBlockDto{
				Type: "diagram",
				Text: strings.TrimSpace(diagBuffer.String()),
			})
			continue
		}

		// Sequential Flow (e.g. Step 1 -> Step 2 -> Step 3)
		if strings.Contains(line, "-->") || strings.Contains(line, "==>") || (strings.Contains(line, "->") && !strings.HasPrefix(line, "- ")) {
			if len(flowArrowRegex.FindAllString(line, -1)) >= 2 {
				blocks = append(blocks, dto.AiBlockDto{
					Type: "visualFlow",
					Text: line,
				})
				continue
			}
		}

		// Horizontal Divider Rules (---, ***, ___)
		if dividerRegex.MatchString(line) {
			flushCallout()
			flushTable()
			flushFormula()
			blocks = append(blocks, dto.AiBlockDto{
				Type: "divider",
				Text: "",
			})
			continue
		}

		// Headings (H1 - H6)
		if strings.HasPrefix(line, "# ") {
			blocks = append(blocks, dto.AiBlockDto{
				Type: "heading1",
				Text: strings.TrimSpace(line[2:]),
			})
		} else if strings.HasPrefix(line, "## ") {
			h2Text := strings.TrimSpace(line[3:])
			lowerH2 := strings.ToLower(h2Text)
			if strings.Contains(lowerH2, "foundation") || strings.Contains(lowerH2, "deep explanation") ||
				strings.Contains(lowerH2, "exam tip") || strings.Contains(lowerH2, "high-yield") ||
				strings.Contains(lowerH2, "misconception") || strings.Contains(lowerH2, "trap") ||
				strings.Contains(lowerH2, "related concept") || strings.Contains(lowerH2, "takeaway") ||
				strings.Contains(lowerH2, "core concept") || strings.Contains(lowerH2, "summary") {
				blocks = append(blocks, dto.AiBlockDto{
					Type: "heading3",
					Text: h2Text,
				})
			} else {
				blocks = append(blocks, dto.AiBlockDto{
					Type: "heading2",
					Text: h2Text,
				})
			}
		} else if strings.HasPrefix(line, "### ") {
			blocks = append(blocks, dto.AiBlockDto{
				Type: "heading3",
				Text: strings.TrimSpace(line[4:]),
			})
		} else if h4to6Regex.MatchString(line) {
			cleanHeading := strings.TrimSpace(h4to6Regex.ReplaceAllString(line, ""))
			blocks = append(blocks, dto.AiBlockDto{
				Type: "heading3",
				Text: cleanHeading,
			})
		} else if (strings.HasPrefix(line, "$$") && strings.HasSuffix(line, "$$") && len(line) > 4) ||
			(strings.HasPrefix(line, `\[`) && strings.HasSuffix(line, `\]`) && len(line) > 4) ||
			strings.HasPrefix(strings.ToLower(line), "formula:") ||
			strings.HasPrefix(strings.ToLower(line), "equation:") {
			inner := line
			if strings.HasPrefix(line, "$$") && strings.HasSuffix(line, "$$") {
				inner = strings.TrimSpace(line[2 : len(line)-2])
			} else if strings.HasPrefix(line, `\[`) && strings.HasSuffix(line, `\]`) {
				inner = strings.TrimSpace(line[2 : len(line)-2])
			}
			if (strings.HasPrefix(line, "$$") || strings.HasPrefix(line, `\[`)) && !isGenuineMath(inner) {
				blocks = append(blocks, dto.AiBlockDto{
					Type: "paragraph",
					Text: inner,
				})
			} else {
				blocks = append(blocks, dto.AiBlockDto{
					Type: "formula",
					Text: line,
				})
			}
		} else if clMatch := checklistRegex.FindStringSubmatch(line); len(clMatch) >= 3 {
			isChecked := strings.ToLower(clMatch[1]) == "x"
			extra := "false"
			if isChecked {
				extra = "true"
			}
			blocks = append(blocks, dto.AiBlockDto{
				Type:  "checklist",
				Text:  clMatch[2],
				Extra: extra,
			})
		} else if nlMatch := numberedRegex.FindStringSubmatch(line); len(nlMatch) >= 3 {
			blocks = append(blocks, dto.AiBlockDto{
				Type:  "numbered",
				Text:  nlMatch[2],
				Extra: nlMatch[1],
			})
		} else if blMatch := bulletRegex.FindStringSubmatch(line); len(blMatch) >= 2 {
			blocks = append(blocks, dto.AiBlockDto{
				Type: "bullet",
				Text: strings.TrimSpace(blMatch[1]),
			})
		} else {
			blocks = append(blocks, dto.AiBlockDto{
				Type: "paragraph",
				Text: line,
			})
		}
	}

	flushCallout()
	flushTable()
	flushFormula()

	if insideCodeBlock && codeBuffer.Len() > 0 {
		content := strings.TrimSpace(codeBuffer.String())
		lang := strings.ToLower(strings.TrimSpace(codeLanguage))
		if isDiagramLanguageOrContent(lang, content) {
			blocks = append(blocks, dto.AiBlockDto{
				Type:  "diagram",
				Text:  content,
				Extra: codeLanguage,
			})
		} else {
			blocks = append(blocks, dto.AiBlockDto{
				Type:  "code",
				Text:  content,
				Extra: codeLanguage,
			})
		}
	}

	// Pre-sanitize LaTeX formatting and raw backslashes on all prose blocks
	// Offloads regex sweeping from the mobile client to Go backend
	for i := range blocks {
		switch blocks[i].Type {
		case "code", "diagram", "formula":
			// Preserve raw code, diagrams, and formulas without modification
		default:
			blocks[i].Text = SanitizeLatex(blocks[i].Text)
		}
	}

	return blocks
}

func isDiagramLanguageOrContent(lang, content string) bool {
	l := strings.ToLower(lang)
	if strings.HasPrefix(l, "diagram") || strings.HasPrefix(l, "sketch") {
		return true
	}
	common := []string{"mermaid", "flowchart", "ascii", "tree", "schema", "cycle", "compare", "mindmap", "hub", "stack", "timeline", "milestone", "handdrawn", "rough", "whiteboard", "excalidraw"}
	for _, c := range common {
		if l == c {
			return true
		}
	}
	if strings.Contains(content, `"nodes"`) && (strings.Contains(content, `"edges"`) || strings.Contains(content, `"relations"`) || strings.Contains(content, `"title"`)) {
		return true
	}
	if strings.Contains(content, "├──") || strings.Contains(content, "└──") || strings.Contains(content, "┌──") || strings.Contains(content, "+--") || strings.Contains(content, "-->") {
		return true
	}
	return false
}

func isStandaloneDiagramText(line string) bool {
	return strings.Contains(line, "├──") || strings.Contains(line, "└──") || strings.HasPrefix(line, "┌") || strings.HasPrefix(line, "+---") || strings.HasPrefix(line, "|   |")
}

func isGenuineMath(text string) bool {
	t := strings.TrimSpace(text)
	if len(t) == 0 {
		return false
	}
	mathIndicators := []string{"\\frac", "\\sqrt", "\\int", "\\sum", "\\prod", "\\vec", "\\partial", "\\alpha", "\\beta", "\\gamma", "\\theta", "\\lambda", "\\Delta", "\\pm", "\\times", "\\le", "\\ge", "\\neq", "^", "_", "=", "+", "-"}
	for _, ind := range mathIndicators {
		if strings.Contains(t, ind) {
			return true
		}
	}
	return false
}

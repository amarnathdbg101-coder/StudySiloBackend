package services

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"

	"github.com/chromedp/chromedp"
	"github.com/jung-kurt/gofpdf"
)

type PDFExportService interface {
	ExportStudyPack(ctx context.Context, userID string, req dto.ExportPDFRequest) ([]byte, error)
	ExportMindMapPoster(ctx context.Context, req dto.ExportMindMapPDFRequest) ([]byte, error)
}

type pdfExportService struct {
	noteRepo repository.NoteRepository
	quizRepo repository.QuizRepository
}

func NewPDFExportService(noteRepo repository.NoteRepository, quizRepo repository.QuizRepository) PDFExportService {
	return &pdfExportService{
		noteRepo: noteRepo,
		quizRepo: quizRepo,
	}
}

func (s *pdfExportService) ExportStudyPack(ctx context.Context, userID string, req dto.ExportPDFRequest) ([]byte, error) {
	title := req.Title
	if title == "" {
		title = "StudySilo Study Guide"
	}
	content := req.Content

	// If note_id provided, fetch note content
	if req.NoteID != "" && s.noteRepo != nil {
		note, err := s.noteRepo.GetByID(ctx, req.NoteID, userID)
		if err == nil && note != nil {
			title = note.Title
			content = note.Content
			if content == "" {
				content = note.Description
			}
		}
	}
	req.Title = title
	req.Content = content

	var quizQuestions []*model.QuizQuestion
	if req.QuizID != "" && s.quizRepo != nil {
		quiz, err := s.quizRepo.GetQuizByID(ctx, req.QuizID, userID)
		if err == nil && quiz != nil {
			quizQuestions = quiz.Questions
		}
	}

	// 1. Try modern headless Chrome (chromedp) for rich vector typography & KaTeX math formulas
	pdfBytes, err := s.exportWithChromedp(ctx, req, quizQuestions)
	if err == nil && len(pdfBytes) > 100 {
		return pdfBytes, nil
	}

	// 2. Graceful fallback to gofpdf if Chrome/Chromium binary is not installed in the environment
	return s.exportWithGofpdf(req, quizQuestions)
}

// chromeSemaphore limits concurrent headless Chrome tasks to 1.
// On a 1 CPU / 1GB RAM budget VPS, running multiple headless Chrome processes simultaneously
// will exhaust RAM and trigger Linux Kernel Out-Of-Memory (OOM) killer.
var chromeSemaphore = make(chan struct{}, 1)

func acquireChromeLock(ctx context.Context) (func(), error) {
	select {
	case chromeSemaphore <- struct{}{}:
		return func() { <-chromeSemaphore }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *pdfExportService) exportWithChromedp(ctx context.Context, req dto.ExportPDFRequest, questions []*model.QuizQuestion) ([]byte, error) {
	cCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	release, err := acquireChromeLock(cCtx)
	if err != nil {
		return nil, err
	}
	defer release()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Headless,
		chromedp.Flag("disable-dev-shm-usage", true),          // Prevents /dev/shm shared memory crashes on Linux VPS
		chromedp.Flag("no-zygote", true),                      // Saves 30-50MB RAM by avoiding zygote process
		chromedp.Flag("mute-audio", true),                     // Disable audio subsystem
		chromedp.Flag("disable-extensions", true),             // Disable extensions
		chromedp.Flag("js-flags", "--max-old-space-size=128"), // Caps V8 heap to 128MB
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(cCtx, opts...)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	htmlContent := buildStudyPackHTML(req, questions)

	err = chromedp.Do(taskCtx,
		chromedp.Navigate("data:text/html;charset=utf-8,"+url.PathEscape(htmlContent)),
	)
	if err != nil {
		return nil, err
	}

	// ⚡ Allow KaTeX auto-render engine to render math formulas into crisp vector SVG before printing
	_ = chromedp.Do(taskCtx, chromedp.Sleep(450*time.Millisecond))

	buf, err := chromedp.Run(taskCtx,
		chromedp.PrintToPDF(
			chromedp.PDFPaper(chromedp.PaperA4),
			chromedp.PDFMargin(0.3),
			chromedp.PDFPrintBackground(),
		),
	)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func renderInline(text string) string {
	escaped := html.EscapeString(text)

	// Bold: **text** -> <strong>text</strong>
	for {
		start := strings.Index(escaped, "**")
		if start == -1 {
			break
		}
		end := strings.Index(escaped[start+2:], "**")
		if end == -1 {
			break
		}
		boldContent := escaped[start+2 : start+2+end]
		escaped = escaped[:start] + "<strong class='note-bold'>" + boldContent + "</strong>" + escaped[start+2+end+2:]
	}

	// Inline Code: `code` -> <code>code</code>
	for {
		start := strings.Index(escaped, "`")
		if start == -1 {
			break
		}
		end := strings.Index(escaped[start+1:], "`")
		if end == -1 {
			break
		}
		codeContent := escaped[start+1 : start+1+end]
		escaped = escaped[:start] + "<code class='inline-code'>" + codeContent + "</code>" + escaped[start+1+end+1:]
	}

	return escaped
}

func parseMarkdownToStudyHTML(content string) string {
	var sb strings.Builder
	lines := strings.Split(content, "\n")

	inCodeBlock := false
	codeLang := ""
	var codeContent strings.Builder

	inTable := false
	var tableHeader []string
	var tableRows [][]string

	flushTable := func() {
		if !inTable || len(tableHeader) == 0 {
			inTable = false
			tableHeader = nil
			tableRows = nil
			return
		}
		sb.WriteString("<div class='table-wrapper'><table class='notes-table'><thead><tr>")
		for _, h := range tableHeader {
			sb.WriteString(fmt.Sprintf("<th>%s</th>", renderInline(strings.TrimSpace(h))))
		}
		sb.WriteString("</tr></thead><tbody>")
		for _, row := range tableRows {
			sb.WriteString("<tr>")
			for _, cell := range row {
				sb.WriteString(fmt.Sprintf("<td>%s</td>", renderInline(strings.TrimSpace(cell))))
			}
			sb.WriteString("</tr>")
		}
		sb.WriteString("</tbody></table></div>")
		inTable = false
		tableHeader = nil
		tableRows = nil
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Code block handling
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				inCodeBlock = false
				sb.WriteString(fmt.Sprintf("<div class='code-block'><div class='code-badge'>%s</div><pre><code>%s</code></pre></div>",
					html.EscapeString(codeLang), html.EscapeString(codeContent.String())))
				codeContent.Reset()
				codeLang = ""
			} else {
				flushTable()
				inCodeBlock = true
				codeLang = strings.ToUpper(strings.TrimPrefix(trimmed, "```"))
				if codeLang == "" {
					codeLang = "CODE / FORMULA"
				}
			}
			continue
		}
		if inCodeBlock {
			codeContent.WriteString(line)
			codeContent.WriteString("\n")
			continue
		}

		// Table handling
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			rawCols := strings.Split(trimmed, "|")
			if len(rawCols) >= 3 {
				var cols []string
				for cIdx := 1; cIdx < len(rawCols)-1; cIdx++ {
					cols = append(cols, rawCols[cIdx])
				}

				// Check if separator line (|---|---|)
				isSep := true
				for _, c := range cols {
					cTrim := strings.TrimSpace(c)
					if !strings.Contains(cTrim, "-") && cTrim != "" {
						isSep = false
						break
					}
				}

				if isSep {
					continue
				}

				if !inTable {
					inTable = true
					tableHeader = cols
				} else {
					tableRows = append(tableRows, cols)
				}
				continue
			}
		} else if inTable {
			flushTable()
		}

		if trimmed == "" {
			sb.WriteString("<div class='spacer'></div>")
			continue
		}

		// Callouts
		if strings.HasPrefix(trimmed, "> ") {
			calloutContent := strings.TrimPrefix(trimmed, "> ")
			classType := "callout-concept"
			title := "💡 Core Academic Concept"
			icon := "💡"

			lower := strings.ToLower(calloutContent)
			if strings.Contains(lower, "trap") || strings.Contains(lower, "warning") || strings.Contains(lower, "pitfall") || strings.Contains(calloutContent, "⚠️") {
				classType = "callout-trap"
				title = "⚠️ Examiner Trap & Common Mistake"
				icon = "⚠️"
			} else if strings.Contains(lower, "formula") || strings.Contains(lower, "equation") || strings.Contains(calloutContent, "📐") {
				classType = "callout-formula"
				title = "📐 Key Formula & Derivation"
				icon = "📐"
			} else if strings.Contains(lower, "tip") || strings.Contains(lower, "shortcut") || strings.Contains(calloutContent, "🎯") {
				classType = "callout-tip"
				title = "🎯 Exam Shortcut & Strategy"
				icon = "🎯"
			}

			cleanText := calloutContent
			cleanText = strings.TrimPrefix(cleanText, "[!NOTE]")
			cleanText = strings.TrimPrefix(cleanText, "[!WARNING]")
			cleanText = strings.TrimPrefix(cleanText, "[!TIP]")

			sb.WriteString(fmt.Sprintf("<div class='callout %s'><div class='callout-header'><span class='callout-icon'>%s</span><span class='callout-title'>%s</span></div><div class='callout-body'>%s</div></div>",
				classType, icon, title, renderInline(cleanText)))
			continue
		}

		// Headings
		if strings.HasPrefix(trimmed, "#### ") {
			sb.WriteString(fmt.Sprintf("<h4 class='note-h4'>%s</h4>", renderInline(strings.TrimPrefix(trimmed, "#### "))))
		} else if strings.HasPrefix(trimmed, "### ") {
			sb.WriteString(fmt.Sprintf("<h3 class='note-h3'>%s</h3>", renderInline(strings.TrimPrefix(trimmed, "### "))))
		} else if strings.HasPrefix(trimmed, "## ") {
			sb.WriteString(fmt.Sprintf("<h2 class='note-h2'>%s</h2>", renderInline(strings.TrimPrefix(trimmed, "## "))))
		} else if strings.HasPrefix(trimmed, "# ") {
			sb.WriteString(fmt.Sprintf("<h1 class='note-h1'>%s</h1>", renderInline(strings.TrimPrefix(trimmed, "# "))))
		} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			sb.WriteString(fmt.Sprintf("<div class='bullet-item'><span class='bullet-dot'>•</span><span class='bullet-text'>%s</span></div>",
				renderInline(strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* "))))
		} else if len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && trimmed[1] == '.' {
			num := trimmed[:2]
			rest := strings.TrimSpace(trimmed[2:])
			sb.WriteString(fmt.Sprintf("<div class='num-item'><span class='num-badge'>%s</span><span class='bullet-text'>%s</span></div>",
				num, renderInline(rest)))
		} else {
			sb.WriteString(fmt.Sprintf("<p class='note-p'>%s</p>", renderInline(trimmed)))
		}
	}
	flushTable()
	return sb.String()
}

func buildStudyPackHTML(req dto.ExportPDFRequest, questions []*model.QuizQuestion) string {
	var bodyHtml strings.Builder

	title := req.Title
	if title == "" {
		title = "StudySilo Academic Notes"
	}
	docType := req.DocumentType
	if docType == "" {
		docType = "Comprehensive Study Pack & Lecture Notes"
	}
	subject := req.Subject
	if subject == "" {
		subject = "Core Curriculum & Theory"
	}
	targetExam := req.TargetExam
	if targetExam == "" {
		targetExam = "Standard Academic Syllabus"
	}
	studentName := req.StudentName
	if studentName == "" {
		studentName = "StudySilo Scholar"
	}
	dateStr := time.Now().Format("02 Jan 2006")

	// Render Chat Turns if present
	if len(req.ChatTurns) > 0 {
		bodyHtml.WriteString("<div class='chat-thread-container'>")
		bodyHtml.WriteString("<div class='section-intro'><span class='section-icon'>💬</span><span class='section-heading'>Interactive Q&A & Lecture Dialogue</span></div>")
		for i, turn := range req.ChatTurns {
			if turn.Role == "user" {
				bodyHtml.WriteString("<div class='chat-card turn-student'>")
				bodyHtml.WriteString(fmt.Sprintf("<div class='chat-card-header'><span class='badge student-badge'>STUDENT QUERY #%d</span>", i/2+1))
				if turn.Timestamp != "" {
					bodyHtml.WriteString(fmt.Sprintf("<span class='chat-time'>%s</span>", html.EscapeString(turn.Timestamp)))
				}
				bodyHtml.WriteString("</div>")
				bodyHtml.WriteString(fmt.Sprintf("<div class='chat-content'>%s</div>", parseMarkdownToStudyHTML(turn.Content)))
				bodyHtml.WriteString("</div>")
			} else {
				bodyHtml.WriteString("<div class='chat-card turn-tutor'>")
				bodyHtml.WriteString("<div class='chat-card-header'><span class='badge tutor-badge'>🎓 AI PROFESSOR • STEP-BY-STEP SOLUTION</span></div>")
				bodyHtml.WriteString(fmt.Sprintf("<div class='chat-content'>%s</div>", parseMarkdownToStudyHTML(turn.Content)))
				bodyHtml.WriteString("</div>")
			}
		}
		bodyHtml.WriteString("</div>")
	}

	// Render Markdown Body Content
	if req.Content != "" {
		bodyHtml.WriteString(parseMarkdownToStudyHTML(req.Content))
	}

	// Render Quiz Questions if present
	if len(questions) > 0 {
		bodyHtml.WriteString("<div class='quiz-section'>")
		bodyHtml.WriteString("<div class='section-intro'><span class='section-icon'>📝</span><span class='section-heading'>Diagnostic Assessment & Review Traps</span></div>")
		for i, q := range questions {
			bodyHtml.WriteString("<div class='quiz-card'>")
			bodyHtml.WriteString(fmt.Sprintf("<div class='quiz-question-header'><span class='q-number'>Q%d</span><span class='q-text'>%s</span></div>", i+1, renderInline(q.QuestionText)))
			bodyHtml.WriteString("<div class='quiz-options'>")
			for optIdx, opt := range q.Options {
				isCorrect := optIdx == q.CorrectOptionIndex
				checkedClass := "opt-neutral"
				marker := fmt.Sprintf("(%c) ", 'A'+optIdx)
				if isCorrect && req.IncludeSolutions {
					checkedClass = "opt-correct"
					marker = "✓ "
				}
				bodyHtml.WriteString(fmt.Sprintf("<div class='opt-item %s'><span class='opt-marker'>%s</span><span class='opt-text'>%s</span></div>",
					checkedClass, marker, renderInline(opt)))
			}
			bodyHtml.WriteString("</div>")

			if req.IncludeSolutions {
				if q.Explanation != "" {
					bodyHtml.WriteString(fmt.Sprintf("<div class='quiz-explanation'><span class='expl-label'>EXPLANATION:</span> %s</div>", renderInline(q.Explanation)))
				}
				if q.TrapWarning != "" {
					bodyHtml.WriteString(fmt.Sprintf("<div class='quiz-trap'><span class='trap-label'>⚠️ EXAMINER TRAP ALERT:</span> %s</div>", renderInline(q.TrapWarning)))
				}
			}
			bodyHtml.WriteString("</div>")
		}
		bodyHtml.WriteString("</div>")
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>%s - StudySilo Notes</title>
<!-- KaTeX Vector Math Rendering -->
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/katex@0.16.8/dist/katex.min.css">
<script defer src="https://cdn.jsdelivr.net/npm/katex@0.16.8/dist/katex.min.js"></script>
<script defer src="https://cdn.jsdelivr.net/npm/katex@0.16.8/dist/contrib/auto-render.min.js"
    onload="renderMathInElement(document.body, {
        delimiters: [
            {left: '$$', right: '$$', display: true},
            {left: '$', right: '$', display: false},
            {left: '\\(', right: '\\)', display: false},
            {left: '\\[', right: '\\]', display: true}
        ]
    });"></script>
<style>
  @page {
    size: A4;
    margin: 14mm 14mm 14mm 14mm;
  }
  * { box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    color: #0F172A;
    background: #FFFFFF;
    margin: 0;
    padding: 0;
    font-size: 13px;
    line-height: 1.6;
    -webkit-print-color-adjust: exact;
    print-color-adjust: exact;
  }

  /* Academic Header Bar */
  .brand-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 2px solid #4F46E5;
    padding-bottom: 10px;
    margin-bottom: 16px;
  }
  .brand-logo {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .brand-icon {
    width: 26px;
    height: 26px;
    background: #4F46E5;
    color: white;
    font-weight: 900;
    font-size: 14px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 6px;
  }
  .brand-name {
    font-size: 19px;
    font-weight: 900;
    color: #0F172A;
    letter-spacing: -0.4px;
  }
  .header-meta {
    font-size: 11px;
    color: #64748B;
    font-weight: 600;
  }

  /* Title Banner */
  .title-banner {
    background: linear-gradient(135deg, #1E1B4B 0%%, #312E81 100%%);
    color: white;
    padding: 18px 22px;
    border-radius: 12px;
    margin-bottom: 18px;
  }
  .title-banner h1 {
    margin: 0;
    font-size: 22px;
    font-weight: 800;
    letter-spacing: -0.3px;
  }
  .meta-pills {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 10px;
  }
  .pill {
    font-size: 10.5px;
    font-weight: 700;
    padding: 3px 10px;
    border-radius: 999px;
    background: rgba(255, 255, 255, 0.18);
    color: white;
    letter-spacing: 0.2px;
  }
  .pill-accent {
    background: #F59E0B;
    color: #0F172A;
  }

  /* Headings */
  .note-h1 { font-size: 18px; font-weight: 800; color: #0F172A; margin: 20px 0 8px 0; border-bottom: 1.5px solid #E2E8F0; padding-bottom: 4px; }
  .note-h2 { font-size: 15.5px; font-weight: 800; color: #1E293B; margin: 16px 0 6px 0; border-left: 4px solid #4F46E5; padding-left: 8px; }
  .note-h3 { font-size: 13.5px; font-weight: 700; color: #4F46E5; margin: 12px 0 4px 0; }
  .note-h4 { font-size: 12.5px; font-weight: 700; color: #334155; margin: 10px 0 4px 0; }
  .note-p { margin: 0 0 8px 0; color: #1E293B; }
  .note-bold { color: #0F172A; font-weight: 700; }
  .inline-code { background: #F1F5F9; color: #4338CA; padding: 1px 5px; border-radius: 4px; font-family: 'Consolas', monospace; font-size: 11.5px; }
  .spacer { height: 6px; }

  /* Bullets & Lists */
  .bullet-item, .num-item { display: flex; align-items: baseline; margin-bottom: 4px; }
  .bullet-dot { color: #4F46E5; font-weight: bold; width: 14px; font-size: 15px; }
  .num-badge { color: #4F46E5; font-weight: 800; font-size: 11.5px; width: 20px; }
  .bullet-text { flex: 1; color: #1E293B; }

  /* Callout Boxes (Harvard / Notion Style) */
  .callout {
    border-radius: 10px;
    padding: 12px 14px;
    margin: 12px 0;
    page-break-inside: avoid;
  }
  .callout-header {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 4px;
    font-weight: 800;
    font-size: 11.5px;
    text-transform: uppercase;
    letter-spacing: 0.3px;
  }
  .callout-body {
    font-size: 12px;
    line-height: 1.5;
  }
  .callout-concept { background: #EEF2FF; border-left: 4px solid #4F46E5; color: #312E81; }
  .callout-trap { background: #FEF2F2; border-left: 4px solid #EF4444; color: #991B1B; }
  .callout-formula { background: #0F172A; color: #38BDF8; border-left: 4px solid #38BDF8; font-family: 'Consolas', monospace; }
  .callout-formula .callout-title { color: #7DD3FC; }
  .callout-tip { background: #ECFDF5; border-left: 4px solid #10B981; color: #065F46; }

  /* Academic Tables */
  .table-wrapper {
    margin: 12px 0;
    border: 1px solid #E2E8F0;
    border-radius: 8px;
    overflow: hidden;
    page-break-inside: avoid;
  }
  .notes-table {
    width: 100%%;
    border-collapse: collapse;
    font-size: 12px;
  }
  .notes-table th {
    background: #0F172A;
    color: #F8FAFC;
    font-weight: 700;
    padding: 9px 12px;
    text-align: left;
    font-size: 11px;
    letter-spacing: 0.4px;
  }
  .notes-table td {
    padding: 8px 12px;
    border-bottom: 1px solid #E2E8F0;
    color: #1E293B;
  }
  .notes-table tr:nth-child(even) td { background: #F8FAFC; }

  /* Code Block */
  .code-block {
    background: #0F172A;
    color: #38BDF8;
    border-radius: 8px;
    padding: 10px 14px;
    margin: 10px 0;
    font-family: 'Consolas', monospace;
    font-size: 11.5px;
    page-break-inside: avoid;
  }
  .code-badge { font-size: 9px; font-weight: 800; color: #94A3B8; text-transform: uppercase; margin-bottom: 4px; }
  .code-block pre { margin: 0; white-space: pre-wrap; word-break: break-all; }

  /* Chat Thread Cards */
  .chat-thread-container { margin: 16px 0; }
  .section-intro { display: flex; align-items: center; gap: 8px; font-size: 14px; font-weight: 800; color: #0F172A; margin-bottom: 10px; }
  .chat-card {
    border-radius: 10px;
    padding: 12px 14px;
    margin-bottom: 10px;
    page-break-inside: avoid;
  }
  .turn-student {
    background: #F8FAFC;
    border: 1px solid #E2E8F0;
    border-left: 4px solid #64748B;
  }
  .turn-tutor {
    background: #FFFFFF;
    border: 1px solid #C7D2FE;
    border-left: 4px solid #4F46E5;
    box-shadow: 0 1px 3px rgba(79, 70, 229, 0.05);
  }
  .chat-card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }
  .badge { font-size: 9.5px; font-weight: 800; padding: 2px 7px; border-radius: 4px; letter-spacing: 0.3px; }
  .student-badge { background: #E2E8F0; color: #334155; }
  .tutor-badge { background: #EEF2FF; color: #4F46E5; }
  .chat-time { font-size: 10px; color: #94A3B8; }

  /* Quiz Section */
  .quiz-section { margin-top: 20px; page-break-inside: avoid; }
  .quiz-card {
    background: white;
    border: 1px solid #E2E8F0;
    border-radius: 10px;
    padding: 12px 14px;
    margin-bottom: 10px;
    page-break-inside: avoid;
  }
  .quiz-question-header { display: flex; gap: 8px; font-size: 13px; font-weight: 700; color: #0F172A; margin-bottom: 8px; }
  .q-number { background: #4F46E5; color: white; padding: 1px 6px; border-radius: 4px; font-size: 11px; height: fit-content; }
  .opt-item { padding: 4px 8px; font-size: 12px; border-radius: 6px; margin-bottom: 3px; }
  .opt-neutral { color: #334155; }
  .opt-correct { background: #DCFCE7; color: #15803D; font-weight: 700; }
  .quiz-explanation { background: #F0FDF4; border-left: 3px solid #16A34A; color: #166534; padding: 6px 10px; font-size: 11.5px; margin-top: 6px; border-radius: 4px; }
  .quiz-trap { background: #FEF2F2; border-left: 3px solid #DC2626; color: #991B1B; padding: 6px 10px; font-size: 11.5px; margin-top: 4px; border-radius: 4px; }
  .expl-label, .trap-label { font-weight: 800; font-size: 10px; }

  /* Footer */
  .doc-footer {
    margin-top: 30px;
    padding-top: 10px;
    border-top: 1px solid #CBD5E1;
    font-size: 10.5px;
    color: #64748B;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
</style>
</head>
<body>
  <!-- Brand Header -->
  <div class="brand-header">
    <div class="brand-logo">
      <div class="brand-icon">S</div>
      <div class="brand-name">StudySilo</div>
    </div>
    <div class="header-meta">%s • %s</div>
  </div>

  <!-- Title Banner -->
  <div class="title-banner">
    <h1>%s</h1>
    <div class="meta-pills">
      <span class="pill pill-accent">%s</span>
      <span class="pill">🎯 %s</span>
      <span class="pill">📚 %s</span>
      <span class="pill">👤 %s</span>
    </div>
  </div>

  <!-- Body Content -->
  <div class="content-body">
    %s
  </div>

  <!-- Footer -->
  <div class="doc-footer">
    <div>StudySilo.in • Autonomous AI Academic Companion</div>
    <div>Personalized Study Pack • High-Yield Revision</div>
  </div>
</body>
</html>`,
		html.EscapeString(title),
		html.EscapeString(docType),
		dateStr,
		html.EscapeString(title),
		html.EscapeString(docType),
		html.EscapeString(targetExam),
		html.EscapeString(subject),
		html.EscapeString(studentName),
		bodyHtml.String(),
	)
}

func (s *pdfExportService) exportWithGofpdf(req dto.ExportPDFRequest, questions []*model.QuizQuestion) ([]byte, error) {
	title := req.Title
	if title == "" {
		title = "StudySilo Study Guide"
	}
	content := req.Content
	if len(req.ChatTurns) > 0 && content == "" {
		var chatText strings.Builder
		for i, t := range req.ChatTurns {
			if t.Role == "user" {
				chatText.WriteString(fmt.Sprintf("\n### Question %d\n%s\n", i/2+1, t.Content))
			} else {
				chatText.WriteString(fmt.Sprintf("\n### Solution\n%s\n", t.Content))
			}
		}
		content = chatText.String()
	}
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 20, 15)
	pdf.SetAutoPageBreak(true, 20)

	pdf.SetHeaderFunc(func() {
		pdf.SetFont("Arial", "I", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 5, "StudySilo | AI-Powered Personalized Study Guide", "", 0, "L", false, 0, "")
		pdf.Ln(8)
	})

	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetFont("Arial", "I", 8)
		pdf.SetTextColor(150, 150, 150)
		pdf.CellFormat(0, 10, fmt.Sprintf("Page %d of {nb}  •  StudySilo.in Academic Companion", pdf.PageNo()), "", 0, "C", false, 0, "")
	})

	pdf.AliasNbPages("{nb}")
	pdf.AddPage()

	// 1. Title Banner
	pdf.SetFillColor(79, 70, 229) // Brand Indigo #4F46E5
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 18)
	pdf.CellFormat(0, 16, "  "+title, "", 1, "L", true, 0, "")
	pdf.Ln(6)

	// 2. Metadata bar
	pdf.SetFillColor(243, 244, 246)
	pdf.SetTextColor(75, 85, 99)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 8, "  Document Type: Academic Revision Pack  |  Format: Summary & Formulas", "", 1, "L", true, 0, "")
	pdf.Ln(6)

	// 3. Document Content rendering
	pdf.SetTextColor(31, 41, 55)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			pdf.Ln(4)
			continue
		}

		if strings.HasPrefix(trimmed, "### ") {
			pdf.SetFont("Arial", "B", 13)
			pdf.SetTextColor(79, 70, 229)
			pdf.MultiCell(0, 7, strings.TrimPrefix(trimmed, "### "), "", "L", false)
			pdf.SetTextColor(31, 41, 55)
			pdf.Ln(2)
		} else if strings.HasPrefix(trimmed, "## ") {
			pdf.SetFont("Arial", "B", 15)
			pdf.SetTextColor(30, 41, 59)
			pdf.MultiCell(0, 8, strings.TrimPrefix(trimmed, "## "), "", "L", false)
			pdf.SetTextColor(31, 41, 55)
			pdf.Ln(3)
		} else if strings.HasPrefix(trimmed, "# ") {
			pdf.SetFont("Arial", "B", 17)
			pdf.SetTextColor(15, 23, 42)
			pdf.MultiCell(0, 9, strings.TrimPrefix(trimmed, "# "), "", "L", false)
			pdf.SetTextColor(31, 41, 55)
			pdf.Ln(4)
		} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			pdf.SetFont("Arial", "", 10)
			bullet := "   • " + strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* ")
			pdf.MultiCell(0, 5, bullet, "", "L", false)
		} else {
			pdf.SetFont("Arial", "", 10)
			pdf.MultiCell(0, 5.5, trimmed, "", "L", false)
		}
	}

	// 4. Render Questions & Solutions section
	if len(questions) > 0 {
		pdf.Ln(8)
		pdf.SetFillColor(241, 245, 249)
		pdf.SetFont("Arial", "B", 13)
		pdf.SetTextColor(15, 23, 42)
		pdf.CellFormat(0, 10, "  Review Questions & Exam Traps", "", 1, "L", true, 0, "")
		pdf.Ln(4)

		for i, q := range questions {
			pdf.SetFont("Arial", "B", 10)
			pdf.SetTextColor(79, 70, 229)
			pdf.MultiCell(0, 6, fmt.Sprintf("Q%d. %s", i+1, q.QuestionText), "", "L", false)

			pdf.SetFont("Arial", "", 9)
			pdf.SetTextColor(55, 65, 81)
			for optIdx, opt := range q.Options {
				marker := "[ ] "
				if optIdx == q.CorrectOptionIndex {
					marker = "[✓] "
				}
				pdf.MultiCell(0, 5, "    "+marker+opt, "", "L", false)
			}

			if q.Explanation != "" {
				pdf.SetFont("Arial", "I", 8.5)
				pdf.SetTextColor(16, 185, 129) // Emerald
				pdf.MultiCell(0, 5, "    Explanation: "+q.Explanation, "", "L", false)
			}
			if q.TrapWarning != "" {
				pdf.SetFont("Arial", "I", 8.5)
				pdf.SetTextColor(239, 68, 68) // Red trap
				pdf.MultiCell(0, 5, "    ⚠️ Trap Alert: "+q.TrapWarning, "", "L", false)
			}
			pdf.Ln(4)
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate PDF output: %w", err)
	}

	return buf.Bytes(), nil
}

func (s *pdfExportService) ExportMindMapPoster(ctx context.Context, req dto.ExportMindMapPDFRequest) ([]byte, error) {
	title := req.Title
	if title == "" {
		title = req.Concept + " 4D Academic Mind Map"
	}
	htmlContent := buildMindMapPosterHTML(title, req.Archetype, req.Summary, req.Nodes, req.Edges)

	// 1. Try headless Chrome (chromedp) for A3/A4 printable wall poster
	cCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	release, err := acquireChromeLock(cCtx)
	if err != nil {
		// Fallback to gofpdf if chrome semaphore timeout
		return s.exportPosterWithGofpdf(title, req.Archetype, req.Summary, req.Nodes)
	}
	defer release()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Headless,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-zygote", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("js-flags", "--max-old-space-size=128"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(cCtx, opts...)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	err = chromedp.Do(taskCtx,
		chromedp.Navigate("data:text/html;charset=utf-8,"+url.PathEscape(htmlContent)),
	)
	if err == nil {
		paper := chromedp.PaperA4
		if strings.EqualFold(req.PaperSize, "A3") {
			paper = chromedp.PaperA3
		}
		buf, pErr := chromedp.Run(taskCtx,
			chromedp.PrintToPDF(
				chromedp.PDFPaper(paper),
				chromedp.PDFMargin(0.3),
				chromedp.PDFPrintBackground(),
			),
		)
		if pErr == nil && len(buf) > 100 {
			return buf, nil
		}
	}

	// 2. Fallback to gofpdf if chromedp is not available
	return s.exportPosterWithGofpdf(title, req.Archetype, req.Summary, req.Nodes)
}

func buildMindMapPosterHTML(title, archetype, summary string, nodes []dto.GraphNode, edges []dto.GraphEdge) string {
	var cardsHtml strings.Builder

	for _, node := range nodes {
		shapeClass := "node-box"
		if node.Shape == "circle" {
			shapeClass = "node-circle"
		} else if node.Shape == "capsule" {
			shapeClass = "node-capsule"
		} else if node.Shape == "diamond" {
			shapeClass = "node-diamond"
		}

		color := node.ColorHex
		if color == "" {
			color = "#4F46E5"
		}

		cardsHtml.WriteString(fmt.Sprintf(`<div class="node-card %s" style="border-left: 4px solid %s;">`, shapeClass, color))
		cardsHtml.WriteString(fmt.Sprintf(`<div class="node-header">`))
		cardsHtml.WriteString(fmt.Sprintf(`<span class="node-title">%s</span>`, html.EscapeString(node.Label)))
		if node.Type != "" {
			cardsHtml.WriteString(fmt.Sprintf(`<span class="badge" style="background:%s20; color:%s;">%s</span>`, color, color, html.EscapeString(node.Type)))
		}
		cardsHtml.WriteString(`</div>`)

		if node.Subtitle != "" {
			cardsHtml.WriteString(fmt.Sprintf(`<div class="node-sub">%s</div>`, html.EscapeString(node.Subtitle)))
		}

		desc := node.Description
		if desc == "" {
			desc = node.KeyTakeaway
		}
		if desc != "" {
			cardsHtml.WriteString(fmt.Sprintf(`<p class="node-desc">%s</p>`, html.EscapeString(desc)))
		}

		// KaTeX / LaTeX Formula Block
		formula := node.FormulaLatex
		if formula == "" && len(node.Formulas) > 0 {
			formula = node.Formulas[0]
		}
		if formula != "" {
			cardsHtml.WriteString(fmt.Sprintf(`<div class="formula-box"><span class="formula-label">FORMULA</span><code>%s</code></div>`, html.EscapeString(formula)))
		}

		// Examiner Trap Warning
		if node.ExamTrap != "" {
			cardsHtml.WriteString(fmt.Sprintf(`<div class="trap-box"><strong>⚠️ Examiner Trap:</strong> %s</div>`, html.EscapeString(node.ExamTrap)))
		}

		// Viva / Interviewer Insight
		if node.VivaInsight != "" {
			cardsHtml.WriteString(fmt.Sprintf(`<div class="viva-box"><strong>💡 Viva Insight:</strong> %s</div>`, html.EscapeString(node.VivaInsight)))
		}

		// Cross-Subject Relational Link
		if node.CrossSubject != "" {
			cardsHtml.WriteString(fmt.Sprintf(`<div class="cross-subject-box"><strong>🌐 Cross-Subject:</strong> %s</div>`, html.EscapeString(node.CrossSubject)))
		}

		cardsHtml.WriteString(`</div>`)
	}

	archDisplay := archetype
	if archDisplay == "" {
		archDisplay = "Academic Concept Map"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  body { font-family: 'Segoe UI', system-ui, -apple-system, sans-serif; margin: 20px; color: #0F172A; background: #FAFAFA; }
  .poster-header { border-bottom: 3px solid #4F46E5; padding-bottom: 12px; margin-bottom: 20px; display: flex; justify-content: space-between; align-items: flex-end; }
  .poster-brand { font-size: 26px; font-weight: 900; color: #4F46E5; letter-spacing: -0.5px; }
  .archetype-pill { background: #EEF2FF; color: #4F46E5; padding: 4px 12px; border-radius: 999px; font-weight: 700; font-size: 12px; text-transform: uppercase; }
  .title-banner { background: linear-gradient(135deg, #1E1B4B 0%%, #312E81 100%%); color: white; padding: 20px 24px; border-radius: 14px; margin-bottom: 22px; }
  .title-banner h1 { margin: 0; font-size: 24px; font-weight: 800; }
  .title-banner p { margin: 8px 0 0 0; opacity: 0.88; font-size: 13px; line-height: 1.4; }
  .grid-container { display: grid; grid-template-columns: repeat(auto-fill, minmax(290px, 1fr)); gap: 16px; }
  .node-card { background: white; border: 1px solid #E2E8F0; border-radius: 12px; padding: 14px; box-shadow: 0 2px 4px rgba(0,0,0,0.04); }
  .node-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
  .node-title { font-weight: 800; font-size: 14px; color: #0F172A; }
  .badge { font-size: 10px; font-weight: 700; padding: 2px 7px; border-radius: 6px; text-transform: uppercase; }
  .node-sub { font-size: 11.5px; font-weight: 600; color: #64748B; margin-bottom: 8px; }
  .node-desc { font-size: 12px; color: #334155; line-height: 1.4; margin: 0 0 10px 0; }
  .formula-box { background: #0F172A; color: #38BDF8; padding: 8px 10px; border-radius: 8px; font-family: 'Consolas', monospace; font-size: 11.5px; margin-bottom: 8px; word-break: break-all; }
  .formula-label { display: block; font-size: 8.5px; font-weight: 800; color: #7DD3FC; margin-bottom: 2px; }
  .trap-box { background: #FEF2F2; border: 1px solid #FECACA; color: #991B1B; padding: 8px 10px; border-radius: 8px; font-size: 11px; margin-bottom: 8px; line-height: 1.35; }
  .viva-box { background: #F0FDF4; border: 1px solid #BBF7D0; color: #166534; padding: 8px 10px; border-radius: 8px; font-size: 11px; margin-bottom: 8px; line-height: 1.35; }
  .cross-subject-box { background: #FAF5FF; border: 1px solid #E9D5FF; color: #6B21A8; padding: 8px 10px; border-radius: 8px; font-size: 11px; margin-bottom: 8px; line-height: 1.35; }
  .poster-footer { margin-top: 30px; padding-top: 12px; border-top: 1px solid #CBD5E1; font-size: 11px; color: #64748B; text-align: center; }
</style>
</head>
<body>
  <div class="poster-header">
    <div class="poster-brand">StudySilo <span style="font-size:16px; font-weight:400; color:#64748B;">4D Mind Map</span></div>
    <div class="archetype-pill">Archetype: %s</div>
  </div>
  <div class="title-banner">
    <h1>%s</h1>
    <p>%s</p>
  </div>
  <div class="grid-container">
    %s
  </div>
  <div class="poster-footer">
    StudySilo.in • High-Yield Academic Mind Map • Designed for Active Recall & Revision Posters
  </div>
</body>
</html>`, html.EscapeString(archDisplay), html.EscapeString(title), html.EscapeString(summary), cardsHtml.String())
}

func (s *pdfExportService) exportPosterWithGofpdf(title, archetype, summary string, nodes []dto.GraphNode) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	pdf.AddPage()
	pdf.SetFont("Arial", "B", 18)
	pdf.SetTextColor(79, 70, 229)
	pdf.CellFormat(0, 10, "StudySilo 4D Mind Map Poster", "", 1, "L", false, 0, "")

	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(15, 23, 42)
	pdf.MultiCell(0, 8, title, "", "L", false)
	pdf.Ln(2)

	if summary != "" {
		pdf.SetFont("Arial", "", 10)
		pdf.SetTextColor(71, 85, 105)
		pdf.MultiCell(0, 5.5, summary, "", "L", false)
		pdf.Ln(4)
	}

	for i, node := range nodes {
		pdf.SetFillColor(248, 250, 252)
		pdf.SetDrawColor(203, 213, 225)
		pdf.SetFont("Arial", "B", 11)
		pdf.SetTextColor(15, 23, 42)
		pdf.CellFormat(0, 8, fmt.Sprintf("%d. %s [%s]", i+1, node.Label, node.Type), "1", 1, "L", true, 0, "")

		if node.Subtitle != "" {
			pdf.SetFont("Arial", "I", 9)
			pdf.SetTextColor(100, 116, 139)
			pdf.MultiCell(0, 5, "   "+node.Subtitle, "", "L", false)
		}

		desc := node.Description
		if desc == "" {
			desc = node.KeyTakeaway
		}
		if desc != "" {
			pdf.SetFont("Arial", "", 9.5)
			pdf.SetTextColor(51, 65, 85)
			pdf.MultiCell(0, 5, "   "+desc, "", "L", false)
		}

		formula := node.FormulaLatex
		if formula == "" && len(node.Formulas) > 0 {
			formula = node.Formulas[0]
		}
		if formula != "" {
			pdf.SetFont("Arial", "B", 9)
			pdf.SetTextColor(14, 165, 233)
			pdf.MultiCell(0, 5, "   Formula: "+formula, "", "L", false)
		}

		if node.ExamTrap != "" {
			pdf.SetFont("Arial", "B", 9)
			pdf.SetTextColor(220, 38, 38)
			pdf.MultiCell(0, 5, "   Trap: "+node.ExamTrap, "", "L", false)
		}

		if node.VivaInsight != "" {
			pdf.SetFont("Arial", "I", 9)
			pdf.SetTextColor(22, 163, 74)
			pdf.MultiCell(0, 5, "   Viva Insight: "+node.VivaInsight, "", "L", false)
		}
		pdf.Ln(3)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate poster PDF: %w", err)
	}
	return buf.Bytes(), nil
}

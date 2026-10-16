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

	var quizQuestions []*model.QuizQuestion
	if req.QuizID != "" && s.quizRepo != nil {
		quiz, err := s.quizRepo.GetQuizByID(ctx, req.QuizID, userID)
		if err == nil && quiz != nil {
			quizQuestions = quiz.Questions
		}
	}

	// 1. Try modern headless Chrome (chromedp) for rich vector typography & KaTeX math formulas
	pdfBytes, err := s.exportWithChromedp(ctx, title, content, quizQuestions)
	if err == nil && len(pdfBytes) > 100 {
		return pdfBytes, nil
	}

	// 2. Graceful fallback to gofpdf if Chrome/Chromium binary is not installed in the environment
	return s.exportWithGofpdf(title, content, quizQuestions)
}
func (s *pdfExportService) exportWithChromedp(ctx context.Context, title, content string, questions []*model.QuizQuestion) ([]byte, error) {
	cCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Headless,
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(cCtx, opts...)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	htmlContent := buildStudyPackHTML(title, content, questions)

	err := chromedp.Do(taskCtx,
		chromedp.Navigate("data:text/html;charset=utf-8,"+url.PathEscape(htmlContent)),
	)
	if err != nil {
		return nil, err
	}

	buf, err := chromedp.Run(taskCtx,
		chromedp.PrintToPDF(
			chromedp.PDFPaper(chromedp.PaperA4),
			chromedp.PDFMargin(0.4),
			chromedp.PDFPrintBackground(),
		),
	)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func buildStudyPackHTML(title, content string, questions []*model.QuizQuestion) string {
	var bodyHtml strings.Builder

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			bodyHtml.WriteString("<br>")
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			bodyHtml.WriteString(fmt.Sprintf("<h3 style='color:#4F46E5; margin-top:14px; margin-bottom:6px;'>%s</h3>", html.EscapeString(strings.TrimPrefix(trimmed, "### "))))
		} else if strings.HasPrefix(trimmed, "## ") {
			bodyHtml.WriteString(fmt.Sprintf("<h2 style='color:#1E293B; margin-top:18px; margin-bottom:8px; border-bottom:1px solid #E2E8F0; padding-bottom:4px;'>%s</h2>", html.EscapeString(strings.TrimPrefix(trimmed, "## "))))
		} else if strings.HasPrefix(trimmed, "# ") {
			bodyHtml.WriteString(fmt.Sprintf("<h1 style='color:#0F172A; margin-top:22px; margin-bottom:10px;'>%s</h1>", html.EscapeString(strings.TrimPrefix(trimmed, "# "))))
		} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			bodyHtml.WriteString(fmt.Sprintf("<li style='margin-left:20px; line-height:1.6;'>%s</li>", html.EscapeString(strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* "))))
		} else {
			bodyHtml.WriteString(fmt.Sprintf("<p style='line-height:1.6; margin-bottom:6px;'>%s</p>", html.EscapeString(trimmed)))
		}
	}

	if len(questions) > 0 {
		bodyHtml.WriteString("<div style='margin-top:28px; padding:16px; background:#F8FAFC; border:1px solid #E2E8F0; border-radius:12px;'>")
		bodyHtml.WriteString("<h2 style='color:#0F172A; margin-top:0;'>Review Questions & Exam Traps</h2>")
		for i, q := range questions {
			bodyHtml.WriteString("<div style='margin-bottom:14px; padding:10px; background:#FFFFFF; border-radius:8px; border-left:4px solid #4F46E5;'>")
			bodyHtml.WriteString(fmt.Sprintf("<p style='font-weight:bold; color:#1E293B;'>Q%d. %s</p>", i+1, html.EscapeString(q.QuestionText)))
			for optIdx, opt := range q.Options {
				checked := "[ ] "
				color := "#475569"
				if optIdx == q.CorrectOptionIndex {
					checked = "✓ "
					color = "#16A34A"
				}
				bodyHtml.WriteString(fmt.Sprintf("<p style='margin:4px 0 4px 16px; color:%s;'>%s %s</p>", color, checked, html.EscapeString(opt)))
			}
			if q.Explanation != "" {
				bodyHtml.WriteString(fmt.Sprintf("<p style='font-size:12px; color:#16A34A; margin-top:6px;'><strong>Explanation:</strong> %s</p>", html.EscapeString(q.Explanation)))
			}
			if q.TrapWarning != "" {
				bodyHtml.WriteString(fmt.Sprintf("<p style='font-size:12px; color:#DC2626; margin-top:4px;'><strong>⚠️ Trap Alert:</strong> %s</p>", html.EscapeString(q.TrapWarning)))
			}
			bodyHtml.WriteString("</div>")
		}
		bodyHtml.WriteString("</div>")
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  body { font-family: 'Segoe UI', Helvetica, Arial, sans-serif; margin: 24px; color: #1E293B; }
  .header { display: flex; justify-content: space-between; align-items: center; border-bottom: 2px solid #4F46E5; padding-bottom: 12px; margin-bottom: 20px; }
  .brand { font-size: 22px; font-weight: 800; color: #4F46E5; }
  .badge { font-size: 11px; background: #EEF2FF; color: #4F46E5; padding: 4px 10px; border-radius: 999px; font-weight: 600; }
  .title-card { background: #4F46E5; color: white; padding: 18px 22px; border-radius: 12px; margin-bottom: 24px; }
  .title-card h1 { margin: 0; font-size: 24px; }
  .footer { margin-top: 36px; padding-top: 12px; border-top: 1px solid #CBD5E1; font-size: 11px; color: #64748B; text-align: center; }
</style>
</head>
<body>
  <div class="header">
    <div class="brand">StudySilo</div>
    <div class="badge">AI-Powered Study Pack</div>
  </div>
  <div class="title-card">
    <h1>%s</h1>
    <p style="margin: 6px 0 0 0; opacity: 0.9; font-size: 12px;">Generated via StudySilo AI Academic Companion</p>
  </div>
  <div class="content">
    %s
  </div>
  <div class="footer">
    StudySilo.in • High-Yield Revision Pack • Empowering Autonomous Learning
  </div>
</body>
</html>`, html.EscapeString(title), bodyHtml.String())
}

func (s *pdfExportService) exportWithGofpdf(title, content string, questions []*model.QuizQuestion) ([]byte, error) {
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

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Headless,
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(cCtx, opts...)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	err := chromedp.Do(taskCtx,
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

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

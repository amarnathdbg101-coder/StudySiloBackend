package services

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/repository"

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

	// Initialize clean A4 portrait PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 20, 15)
	pdf.SetAutoPageBreak(true, 20)

	// Header and Footer Setup
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

	// 4. If QuizID is provided, render Questions & Solutions section
	if req.QuizID != "" && s.quizRepo != nil {
		quiz, err := s.quizRepo.GetQuizByID(ctx, req.QuizID, userID)
		if err == nil && quiz != nil && len(quiz.Questions) > 0 {
			pdf.Ln(8)
			pdf.SetFillColor(241, 245, 249)
			pdf.SetFont("Arial", "B", 13)
			pdf.SetTextColor(15, 23, 42)
			pdf.CellFormat(0, 10, "  Review Questions & Exam Traps", "", 1, "L", true, 0, "")
			pdf.Ln(4)

			for i, q := range quiz.Questions {
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
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate PDF output: %w", err)
	}

	return buf.Bytes(), nil
}

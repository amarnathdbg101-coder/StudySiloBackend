package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/services"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse"
)

type ExportController struct {
	pdfExportService services.PDFExportService
}

func NewExportController(pdfExportService services.PDFExportService) *ExportController {
	return &ExportController{pdfExportService: pdfExportService}
}

// ExportPDF streams a branded study PDF for the Flutter PDF Viewer or web download
func (c *ExportController) ExportPDF(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	var req dto.ExportPDFRequest

	if r.Method == http.MethodPost {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	// Also check query parameters
	if qNote := r.URL.Query().Get("note_id"); qNote != "" {
		req.NoteID = qNote
	}
	if qQuiz := r.URL.Query().Get("quiz_id"); qQuiz != "" {
		req.QuizID = qQuiz
	}
	if qTitle := r.URL.Query().Get("title"); qTitle != "" {
		req.Title = qTitle
	}
	if qSub := r.URL.Query().Get("subject"); qSub != "" {
		req.Subject = qSub
	}
	if qExam := r.URL.Query().Get("target_exam"); qExam != "" {
		req.TargetExam = qExam
	}
	if qDocType := r.URL.Query().Get("document_type"); qDocType != "" {
		req.DocumentType = qDocType
	}
	if qStudent := r.URL.Query().Get("student_name"); qStudent != "" {
		req.StudentName = qStudent
	}
	if qSol := r.URL.Query().Get("include_solutions"); qSol == "true" || qSol == "1" {
		req.IncludeSolutions = true
	}

	if req.Title == "" {
		req.Title = "StudySilo Study Material"
	}

	pdfBytes, err := c.pdfExportService.ExportStudyPack(r.Context(), userID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	safeTitle := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, req.Title)
	if len(safeTitle) > 40 {
		safeTitle = safeTitle[:40]
	}
	filename := fmt.Sprintf("StudySilo_%s.pdf", safeTitle)

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

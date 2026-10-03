package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

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

	if req.Title == "" {
		req.Title = "StudySilo Study Material"
	}

	pdfBytes, err := c.pdfExportService.ExportStudyPack(r.Context(), userID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=\"studysilo_study_pack.pdf\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

package routes

import (
	"net/http"
	"time"

	_ "goProjectStructure/docs"
	"goProjectStructure/internal/handler/controller"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/handler/services"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse/ai"
	"goProjectStructure/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"go.uber.org/zap"
)

// RouteSetup initializes and wires all middlewares and endpoint routes
func RouteSetup(db *pgxpool.Pool, logger *zap.Logger) *chi.Mux {
	r := chi.NewRouter()

	cfg := utils.MustLoad()

	// Global Middlewares Pipeline (Order: RequestID -> Recoverer -> Security -> Logger -> CORS -> RateLimit)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer(logger))
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.Logger)
	r.Use(middleware.CORS)
	limiter := middleware.NewIPRateLimiter(100, time.Minute)
	r.Use(limiter.Middleware())

	// 1. Interactive Swagger Documentation UI
	r.Get("/swagger/*", httpSwagger.WrapHandler)
	r.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
	})

	// 2. Health & Monitoring Probes
	healthCtrl := controller.NewHealthController(db)
	r.Get("/health", healthCtrl.Check)
	r.Get("/health/live", healthCtrl.Live)
	r.Get("/health/ready", healthCtrl.Ready)

	// 3. Domain Repositories
	userRepo := repository.NewUserRepo(db, logger)
	courseRepo := repository.NewCourseRepo(db, logger)
	folderRepo := repository.NewFolderRepo(db, logger)
	noteRepo := repository.NewNoteRepo(db, logger)
	chatRepo := repository.NewChatRepo(db, logger)
	cognitiveRepo := repository.NewCognitiveRepo(db, logger)
	quizRepo := repository.NewQuizRepo(db, logger)
	syncRepo := repository.NewSyncRepo(db, logger)

	// 4. Domain & AI Services
	aiOrchestrator := ai.NewOrchestrator(cfg.GeminiAPIKeys, cfg.OpenAIAPIKeys, cfg.ClaudeAPIKeys)
	authService := services.NewAuthService(userRepo)
	academicService := services.NewAcademicService(courseRepo, folderRepo, noteRepo)
	aiTutorService := services.NewAITutorService(aiOrchestrator, chatRepo)
	notesAnalyzerService := services.NewNotesAnalyzerService(aiOrchestrator, noteRepo)
	diagramService := services.NewDiagramService(aiOrchestrator)
	quizService := services.NewQuizService(quizRepo, cognitiveRepo, aiOrchestrator)
	syncService := services.NewSyncService(syncRepo)
	pdfExportService := services.NewPDFExportService(noteRepo, quizRepo)

	// 5. Domain Controllers
	authCtrl := controller.NewAuthController(authService)
	academicCtrl := controller.NewAcademicController(academicService)
	aiCtrl := controller.NewAIController(aiTutorService, notesAnalyzerService, diagramService)
	quizCtrl := controller.NewQuizController(quizService)
	syncCtrl := controller.NewSyncController(syncService)
	exportCtrl := controller.NewExportController(pdfExportService)

	// 6. API v1 Routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public Auth routes
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authCtrl.Register)
			r.Post("/login", authCtrl.Login)

			// Protected Auth routes
			r.Group(func(r chi.Router) {
				r.Use(middleware.JWTAuth(cfg.JwtSecret))
				r.Get("/profile", authCtrl.Me)
			})
		})

		// Protected Academic routes (Courses, Folders, Notes)
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuth(cfg.JwtSecret))

			// Courses
			r.Route("/courses", func(r chi.Router) {
				r.Get("/", academicCtrl.ListCourses)
				r.Post("/", academicCtrl.CreateCourse)
				r.Get("/{id}", academicCtrl.GetCourse)
				r.Put("/{id}", academicCtrl.UpdateCourse)
				r.Delete("/{id}", academicCtrl.DeleteCourse)
				r.Get("/{courseId}/folders/tree", academicCtrl.GetFolderTree)
			})

			// Folders
			r.Route("/folders", func(r chi.Router) {
				r.Post("/", academicCtrl.CreateFolder)
				r.Get("/{id}", academicCtrl.GetFolder)
				r.Put("/{id}", academicCtrl.UpdateFolder)
				r.Delete("/{id}", academicCtrl.DeleteFolder)
			})

			// Notes
			r.Route("/notes", func(r chi.Router) {
				r.Get("/", academicCtrl.ListNotes)
				r.Post("/", academicCtrl.CreateNote)
				r.Get("/{id}", academicCtrl.GetNote)
				r.Put("/{id}", academicCtrl.UpdateNote)
				r.Delete("/{id}", academicCtrl.DeleteNote)
			})

			// AI Tutor, Notes Analyzer & Diagram Engine
			r.Route("/ai", func(r chi.Router) {
				r.Post("/doubt", aiCtrl.SolveDoubt)
				r.Get("/history", aiCtrl.GetChatHistory)
				r.Post("/analyze-notes", aiCtrl.AnalyzeNotes)
				r.Post("/diagram", aiCtrl.GenerateDiagram)
			})

			// Adaptive Quizzes
			r.Route("/quizzes", func(r chi.Router) {
				r.Get("/", quizCtrl.ListQuizzes)
				r.Post("/generate", quizCtrl.GenerateQuiz)
				r.Get("/{id}", quizCtrl.GetQuiz)
				r.Post("/{id}/submit", quizCtrl.SubmitQuiz)
			})

			// Student Cognitive Memory Profile
			r.Route("/profile/cognitive", func(r chi.Router) {
				r.Get("/", quizCtrl.GetCognitiveProfile)
				r.Put("/", quizCtrl.UpdateCognitiveProfile)
			})

			// Offline-First Delta Sync for Flutter Clients
			r.Get("/sync", syncCtrl.Sync)

			// Branded Study PDF Export
			r.Route("/export", func(r chi.Router) {
				r.Get("/pdf", exportCtrl.ExportPDF)
				r.Post("/pdf", exportCtrl.ExportPDF)
			})
		})
	})

	return r
}

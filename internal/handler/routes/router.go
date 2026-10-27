// Package routes defines the HTTP routing and middleware setup for the application. It initializes the Chi router, configures global middlewares (such as request ID, recovery, security headers, logging, CORS, and rate limiting), and sets up API endpoints for authentication, AI learning features, quizzes, and cognitive profile management. The RouteSetup function wires together controllers, services, and repositories to handle incoming requests and provide appropriate responses.
package routes

import (
	"net/http"
	"time"

	_ "goProjectStructure/docs"
	"goProjectStructure/internal/handler/controller"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/handler/services"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse"
	"goProjectStructure/internal/reuse/ai"
	"goProjectStructure/internal/utils"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"go.uber.org/zap"
)

// RouteSetup initializes and wires all middlewares and endpoint routes
func RouteSetup(db *pgxpool.Pool, logger *zap.Logger) *chi.Mux {
	r := chi.NewRouter()

	cfg := utils.MustLoad()

	// Global Middlewares Pipeline (Order: RequestID -> Recoverer -> GetHead -> Compress -> ResponseTime -> Security -> Logger -> Idempotency -> CORS -> RateLimit)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer(logger))
	r.Use(chimiddleware.GetHead)
	r.Use(chimiddleware.Compress(5, "application/json", "text/html", "text/plain", "application/javascript", "text/css"))
	r.Use(middleware.ResponseTimeHeader)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.ZapLogger(logger))
	r.Use(middleware.IdempotencyMiddleware(10 * time.Minute))
	r.Use(middleware.CORS)
	r.Use(middleware.AILanguage)
	limiter := middleware.NewIPRateLimiter(100, time.Minute, cfg.TrustedProxies...)
	r.Use(limiter.Middleware())

	// 0. Root Ping Endpoint for Uptime Monitors (UptimeRobot / Cron-Job) & Service Info
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		reuse.Success(w, "StudySilo API is operational", map[string]interface{}{
			"status": "healthy",
			"docs":   "/swagger/index.html",
			"health": "/health",
		})
	})

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
	r.Get("/health/metrics", healthCtrl.Metrics)

	// 3. Domain Repositories (AI, Cognitive Profile, Chat, User, Academic & Sync)
	userRepo := repository.NewUserRepo(db, logger)
	chatRepo := repository.NewChatRepo(db, logger)
	cognitiveRepo := repository.NewCognitiveRepo(db, logger)
	quizRepo := repository.NewQuizRepo(db, logger)
	studyPlanRepo := repository.NewStudyPlanRepo(db, logger)
	aiRoadmapRepo := repository.NewAIRoadmapRepo(db, logger) // AI roadmaps ke liye
	courseRepo := repository.NewCourseRepo(db, logger)
	folderRepo := repository.NewFolderRepo(db, logger)
	noteRepo := repository.NewNoteRepo(db, logger)
	syncRepo := repository.NewSyncRepo(db, logger)
	mindMapRepo := repository.NewMindMapRepo(db, logger)
	flashcardRepo := repository.NewFlashcardRepo(db, logger)
	doubtCacheRepo := repository.NewDoubtCacheRepo(db, logger)

	// 4. Domain & AI Services
	aiOrchestrator := ai.NewOrchestrator(cfg.GeminiAPIKeys, cfg.OpenAIAPIKeys, cfg.ClaudeAPIKeys)
	authService := services.NewAuthService(userRepo)
	aiTutorService := services.NewAITutorService(aiOrchestrator, chatRepo, cognitiveRepo).WithDoubtCache(doubtCacheRepo)
	notesAnalyzerService := services.NewNotesAnalyzerService(aiOrchestrator, noteRepo)
	pdfExportService := services.NewPDFExportService(noteRepo, quizRepo)
	diagramService := services.NewDiagramService(aiOrchestrator, mindMapRepo, quizRepo, flashcardRepo, pdfExportService)
	flashcardService := services.NewFlashcardService(flashcardRepo, aiOrchestrator, logger)
	quizService := services.NewQuizService(quizRepo, cognitiveRepo, aiOrchestrator)
	studyPlanService := services.NewStudyPlanService(studyPlanRepo, aiRoadmapRepo, aiOrchestrator)
	academicService := services.NewAcademicService(courseRepo, folderRepo, noteRepo)
	syncService := services.NewSyncService(syncRepo)
	cognitiveService := services.NewCognitiveService(cognitiveRepo, chatRepo, aiOrchestrator, logger)
	neuralTTSService := services.NewNeuralTTSService("uploads/tts_cache")

	// 5. Domain Controllers
	authCtrl := controller.NewAuthController(authService)
	aiCtrl := controller.NewAIController(aiTutorService, notesAnalyzerService, diagramService, flashcardService)
	quizCtrl := controller.NewQuizController(quizService)
	cognitiveCtrl := controller.NewCognitiveController(cognitiveService)
	studyPlanCtrl := controller.NewStudyPlanController(studyPlanService)
	academicCtrl := controller.NewAcademicController(academicService)
	syncCtrl := controller.NewSyncController(syncService)
	exportCtrl := controller.NewExportController(pdfExportService)
	ttsCtrl := controller.NewTTSController(neuralTTSService)

	// 6. API v1 Pure AI Routes
	r.Route("/api/v1", func(r chi.Router) {
		// Studio Neural TTS Endpoint (100% Free Lifetime Real Voice)
		r.Route("/tts", func(r chi.Router) {
			r.Post("/synthesize", ttsCtrl.Synthesize)
			r.Get("/synthesize", ttsCtrl.Synthesize)
		})
		// Public Auth routes
		r.Route("/auth", func(r chi.Router) {
			r.With(middleware.AuthRateLimiter.Middleware()).Post("/register", authCtrl.Register)
			r.With(middleware.AuthRateLimiter.Middleware()).Post("/login", authCtrl.Login)

			// Protected Auth routes
			r.Group(func(r chi.Router) {
				r.Use(middleware.JWTAuth(cfg.JwtSecret))
				r.Get("/profile", authCtrl.Me)
				r.Post("/logout", authCtrl.Logout)
			})
		})

		// AI Learning Routes (Doubt solving, diagrams, history, notes analysis)
		r.Route("/ai", func(r chi.Router) {
			// Public / Guest & Authenticated AI Learning Routes (Optional Auth)
			r.Group(func(r chi.Router) {
				r.Use(middleware.OptionalJWTAuth(cfg.JwtSecret))
				r.Post("/doubt", aiCtrl.SolveDoubt)
				r.Post("/doubt/stream", aiCtrl.SolveDoubtStream)
				r.Get("/history", aiCtrl.GetChatHistory)
				r.Get("/sessions", aiCtrl.ListChatSessions)
				r.Get("/sessions/{id}", aiCtrl.GetChatSessionMessages)
				r.Post("/diagram", aiCtrl.GenerateDiagram)
				r.Post("/diagram/expand-node", aiCtrl.ExpandMindMapNode)
				r.Post("/diagram/generate-quiz", aiCtrl.GenerateQuizFromMindMap)
				r.Post("/diagram/generate-flashcards", aiCtrl.GenerateFlashcardsFromMindMap)
				r.Post("/diagram/export-pdf", aiCtrl.ExportMindMapPDF)
				r.Post("/flashcards/review", aiCtrl.ReviewFlashcard)
				r.Post("/flashcards/generate", aiCtrl.GenerateFlashcardDeck)
			})

			// Strictly Protected AI endpoints (Requires JWT)
			r.Group(func(r chi.Router) {
				r.Use(middleware.JWTAuth(cfg.JwtSecret))
				r.Post("/sessions", aiCtrl.CreateChatSession)
				r.Delete("/sessions/{id}/messages/{messageID}", aiCtrl.DeleteChatTurn)
				r.Patch("/sessions/{id}", aiCtrl.UpdateChatSession)
				r.Delete("/sessions/{id}", aiCtrl.DeleteChatSession)
				r.Post("/analyze-notes", aiCtrl.AnalyzeNotes)

				// Mind Maps Persistence & CRUD (Obsidian/Heptabase Canvas)
				r.Post("/mindmaps", aiCtrl.SaveMindMap)
				r.Get("/mindmaps", aiCtrl.ListMindMaps)
				r.Get("/mindmaps/{id}", aiCtrl.GetMindMap)
				r.Get("/mindmaps/{id}/export-pdf", aiCtrl.ExportMindMapPDFByID)
				r.Put("/mindmaps/{id}", aiCtrl.UpdateMindMap)
				r.Delete("/mindmaps/{id}", aiCtrl.DeleteMindMap)
				r.Patch("/mindmaps/{id}/pin", aiCtrl.TogglePinMindMap)

				// Flashcard Decks Persistence & CRUD (Anki-style Spaced Repetition)
				r.Post("/flashcards/decks", aiCtrl.SaveFlashcardDeck)
				r.Get("/flashcards/decks", aiCtrl.ListFlashcardDecks)
				r.Get("/flashcards/decks/{id}", aiCtrl.GetFlashcardDeck)
				r.Put("/flashcards/decks/{id}", aiCtrl.UpdateFlashcardDeck)
				r.Delete("/flashcards/decks/{id}", aiCtrl.DeleteFlashcardDeck)
				r.Patch("/flashcards/decks/{id}/pin", aiCtrl.TogglePinFlashcardDeck)
			})
		})

		// Standalone Flashcards Route for Direct Access
		r.Route("/flashcards", func(r chi.Router) {
			r.With(middleware.OptionalJWTAuth(cfg.JwtSecret)).Post("/generate", aiCtrl.GenerateFlashcardDeck)
			r.With(middleware.OptionalJWTAuth(cfg.JwtSecret)).Post("/review", aiCtrl.ReviewFlashcard)
			r.Group(func(r chi.Router) {
				r.Use(middleware.JWTAuth(cfg.JwtSecret))
				r.Get("/decks", aiCtrl.ListFlashcardDecks)
				r.Post("/decks", aiCtrl.SaveFlashcardDeck)
				r.Get("/decks/{id}", aiCtrl.GetFlashcardDeck)
				r.Put("/decks/{id}", aiCtrl.UpdateFlashcardDeck)
				r.Delete("/decks/{id}", aiCtrl.DeleteFlashcardDeck)
				r.Patch("/decks/{id}/pin", aiCtrl.TogglePinFlashcardDeck)
			})
		})

		// Adaptive AI Quizzes (Guest & Authenticated)
		r.Route("/quizzes", func(r chi.Router) {
			r.Use(middleware.OptionalJWTAuth(cfg.JwtSecret))
			r.Get("/", quizCtrl.ListQuizzes)
			r.Post("/generate", quizCtrl.GenerateQuiz)
			r.Get("/attempts", quizCtrl.ListAttempts)
			r.Get("/{id}", quizCtrl.GetQuiz)
			r.Delete("/{id}", quizCtrl.DeleteQuiz)
			r.Get("/{id}/attempts", quizCtrl.ListAttempts)
			r.Post("/{id}/submit", quizCtrl.SubmitQuiz)
		})

		// Student Cognitive AI Memory Profile & Mastery Hub
		r.Route("/profile/cognitive", func(r chi.Router) {
			r.With(middleware.OptionalJWTAuth(cfg.JwtSecret)).Get("/", cognitiveCtrl.GetProfile)
			r.With(middleware.OptionalJWTAuth(cfg.JwtSecret)).Get("/smart-context", cognitiveCtrl.GetSmartContext)
			r.With(middleware.OptionalJWTAuth(cfg.JwtSecret)).Put("/", cognitiveCtrl.UpdateGoals)
			r.With(middleware.OptionalJWTAuth(cfg.JwtSecret)).Post("/diagnose", cognitiveCtrl.Diagnose)

			// Authenticated Topic Mastery & Action Drills
			r.Group(func(r chi.Router) {
				r.Use(middleware.JWTAuth(cfg.JwtSecret))
				r.Post("/topics/promote", cognitiveCtrl.PromoteTopic)
				r.Post("/topics/demote", cognitiveCtrl.DemoteTopic)
				r.Delete("/topics", cognitiveCtrl.DeleteTopic)
				r.Post("/activity", cognitiveCtrl.RecordActivity)
			})
		})

		// Study Planner & Spaced Repetition Task Roadmap
		r.Route("/study-plans", func(r chi.Router) {
			r.With(middleware.OptionalJWTAuth(cfg.JwtSecret)).Post("/generate", studyPlanCtrl.GenerateAIPlan)
			r.Group(func(r chi.Router) {
				r.Use(middleware.JWTAuth(cfg.JwtSecret))
				r.Get("/", studyPlanCtrl.List)
				r.Post("/", studyPlanCtrl.Create)
				r.Patch("/{id}", studyPlanCtrl.Update)
				r.Delete("/{id}", studyPlanCtrl.Delete)
			})
		})

		// AI Exam Study Roadmaps - CRUD (Save/List/Get/Update/Delete generated roadmaps)
		r.Route("/roadmaps", func(r chi.Router) {
			r.Use(middleware.JWTAuth(cfg.JwtSecret))
			r.Get("/", studyPlanCtrl.ListRoadmaps)       // Saare roadmaps fetch karo
			r.Post("/", studyPlanCtrl.SaveRoadmap)        // Naya roadmap save karo
			r.Get("/{id}", studyPlanCtrl.GetRoadmap)      // Ek roadmap fetch karo
			r.Patch("/{id}", studyPlanCtrl.UpdateRoadmap) // Roadmap update karo
			r.Delete("/{id}", studyPlanCtrl.DeleteRoadmap) // Roadmap delete karo
		})

		// Focus Study Minutes & Streak Tracker
		r.With(middleware.JWTAuth(cfg.JwtSecret)).Post("/profile/study-time", studyPlanCtrl.RecordStudyTime)

		// Academic Entities (Courses, Folders, Notes with Full-Text Search)
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuth(cfg.JwtSecret))

			r.Route("/courses", func(r chi.Router) {
				r.Get("/", academicCtrl.ListCourses)
				r.Post("/", academicCtrl.CreateCourse)
				r.Get("/{id}", academicCtrl.GetCourse)
				r.Put("/{id}", academicCtrl.UpdateCourse)
				r.Delete("/{id}", academicCtrl.DeleteCourse)
			})

			r.Route("/folders", func(r chi.Router) {
				r.Post("/", academicCtrl.CreateFolder)
				r.Get("/{id}", academicCtrl.GetFolder)
				r.Get("/tree/{courseId}", academicCtrl.GetFolderTree)
				r.Put("/{id}", academicCtrl.UpdateFolder)
				r.Delete("/{id}", academicCtrl.DeleteFolder)
			})

			r.Route("/notes", func(r chi.Router) {
				r.Get("/", academicCtrl.ListNotes)
				r.Post("/", academicCtrl.CreateNote)
				r.Get("/{id}", academicCtrl.GetNote)
				r.Put("/{id}", academicCtrl.UpdateNote)
				r.Delete("/{id}", academicCtrl.DeleteNote)
			})

			// Offline Delta Sync for Mobile Flutter Client
			r.Get("/sync", syncCtrl.Sync)

			// High-Fidelity PDF Export (Chromedp / Vector PDF)
			r.Post("/export/pdf", exportCtrl.ExportPDF)
			r.Get("/export/pdf", exportCtrl.ExportPDF)
		})
	})

	return r
}

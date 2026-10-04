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

	// 3. Domain Repositories (AI, Cognitive Profile, Chat & User)
	userRepo := repository.NewUserRepo(db, logger)
	chatRepo := repository.NewChatRepo(db, logger)
	cognitiveRepo := repository.NewCognitiveRepo(db, logger)
	quizRepo := repository.NewQuizRepo(db, logger)

	// 4. Domain & AI Services
	aiOrchestrator := ai.NewOrchestrator(cfg.GeminiAPIKeys, cfg.OpenAIAPIKeys, cfg.ClaudeAPIKeys)
	authService := services.NewAuthService(userRepo)
	aiTutorService := services.NewAITutorService(aiOrchestrator, chatRepo, cognitiveRepo)
	notesAnalyzerService := services.NewNotesAnalyzerService(aiOrchestrator, nil)
	diagramService := services.NewDiagramService(aiOrchestrator)
	quizService := services.NewQuizService(quizRepo, cognitiveRepo, aiOrchestrator)

	// 5. Domain Controllers
	authCtrl := controller.NewAuthController(authService)
	aiCtrl := controller.NewAIController(aiTutorService, notesAnalyzerService, diagramService)
	quizCtrl := controller.NewQuizController(quizService)

	// 6. API v1 Pure AI Routes
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

		// AI Learning Routes (Doubt solving, diagrams, history, notes analysis)
		r.Route("/ai", func(r chi.Router) {
			// Public / Guest & Authenticated AI Learning Routes (Optional Auth)
			r.Group(func(r chi.Router) {
				r.Use(middleware.OptionalJWTAuth(cfg.JwtSecret))
				r.Post("/doubt", aiCtrl.SolveDoubt)
				r.Post("/diagram", aiCtrl.GenerateDiagram)
			})

			// Strictly Protected AI endpoints (Requires JWT)
			r.Group(func(r chi.Router) {
				r.Use(middleware.JWTAuth(cfg.JwtSecret))
				r.Get("/history", aiCtrl.GetChatHistory)
				r.Post("/analyze-notes", aiCtrl.AnalyzeNotes)
			})
		})

		// Protected Learning Routes (Requires Authentication)
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuth(cfg.JwtSecret))

			// Adaptive AI Quizzes
			r.Route("/quizzes", func(r chi.Router) {
				r.Get("/", quizCtrl.ListQuizzes)
				r.Post("/generate", quizCtrl.GenerateQuiz)
				r.Get("/{id}", quizCtrl.GetQuiz)
				r.Post("/{id}/submit", quizCtrl.SubmitQuiz)
			})

			// Student Cognitive AI Memory Profile
			r.Route("/profile/cognitive", func(r chi.Router) {
				r.Get("/", quizCtrl.GetCognitiveProfile)
				r.Put("/", quizCtrl.UpdateCognitiveProfile)
			})
		})
	})

	return r
}

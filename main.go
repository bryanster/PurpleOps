package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/config"
	"github.com/bryanster/purpleops/internal/db"
	"github.com/bryanster/purpleops/internal/handler"
	"github.com/bryanster/purpleops/internal/render"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg := config.LoadConfig()
	db.InitDB(cfg)
	ssoEnabled := cfg.OAuthEnabled || cfg.SAMLEnabled
	auth.InitSessions(cfg.SecretKey, ssoEnabled)
	render.InitTemplates()

	// Initialise SSO providers.
	if cfg.OAuthEnabled {
		handler.InitOAuth(cfg)
		log.Println("OAuth SSO enabled: provider=" + cfg.OAuthProviderName)
	}
	if cfg.SAMLEnabled {
		if err := handler.InitSAML(cfg); err != nil {
			log.Fatalf("Failed to initialize SAML: %v", err)
		}
		log.Println("SAML SSO enabled")
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Static files
	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// Auth routes (no auth required)
	r.Get("/login", handler.HandleLogin)
	r.Post("/login", handler.HandleLoginPost)
	r.Get("/logout", handler.HandleLogout)

	// OAuth SSO routes
	r.Get("/auth/oauth/login", handler.HandleOAuthLogin)
	r.Get("/auth/oauth/callback", handler.HandleOAuthCallback)

	// SAML SSO routes
	r.Get("/auth/saml/login", handler.HandleSAMLLogin)
	r.Post("/auth/saml/acs", handler.HandleSAMLACS)
	r.Get("/auth/saml/metadata", handler.HandleSAMLMetadata)

	// All authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(auth.AuthRequired)

		// Home / index
		r.Get("/", handler.HandleIndex)
		r.Get("/index", handler.HandleIndex)

		// Password management
		r.Get("/password/change", handler.HandlePasswordChange)
		r.Post("/password/change", handler.HandlePasswordChangePost)
		r.Get("/password/changed", handler.HandlePasswordChanged)

		// MFA routes
		r.Get("/mfa/register", handler.HandleMFARegister)
		r.Post("/mfa/register", handler.HandleMFARegisterPost)
		r.Get("/mfa/verify", handler.HandleMFAVerify)
		r.Post("/mfa/verify", handler.HandleMFAVerifyPost)

		// Assessment CRUD
		r.Post("/assessment", handler.HandleNewAssessment)
		r.Route("/assessment/{id}", func(r chi.Router) {
			r.Use(auth.UserAssignedAssessment)
			r.Get("/", handler.HandleLoadAssessment)
			r.Post("/", handler.HandleEditAssessment)
			r.Delete("/", handler.HandleDeleteAssessment)

			// Assessment utils
			r.Post("/multi/{field}", handler.HandleAssessmentMulti)
			r.Get("/navigator", handler.HandleAssessmentNavigator)
			r.Get("/stats", handler.HandleAssessmentStats)
			r.Get("/assessment_hexagons.svg", handler.HandleAssessmentHexagons)

			// Assessment export
			r.Get("/export/{filetype}", handler.HandleExportAssessment)
			r.Get("/export/campaign", handler.HandleExportCampaign)
			r.Get("/export/templates", handler.HandleExportTestcases)
			r.Post("/export/report", handler.HandleExportReport)
			r.Get("/export/navigator", handler.HandleExportNavigator)
			r.Get("/export/entire", handler.HandleExportEntire)

			// Assessment import
			r.Post("/import/template", handler.HandleImportTemplate)
			r.Post("/import/navigator", handler.HandleImportNavigator)
			r.Post("/import/campaign", handler.HandleImportCampaign)
		})

		// Assessment import entire (no assessment ID in URL)
		r.Post("/assessment/import/entire", handler.HandleImportEntire)

		// Testcase routes
		r.Route("/testcase/{id}", func(r chi.Router) {
			r.Use(auth.UserAssignedAssessment)
			r.Get("/", handler.HandleLoadTestCase)
			r.Post("/", handler.HandleSaveTestCase)
			r.Post("/single", handler.HandleNewTestCase)
			r.Get("/toggle-visibility", handler.HandleToggleVisibility)
			r.Get("/clone", handler.HandleCloneTestCase)
			r.Get("/delete", handler.HandleDeleteTestCase)
			r.Delete("/evidence/{colour}/{file}", handler.HandleDeleteEvidence)
			r.Get("/evidence/{file}", handler.HandleFetchEvidence)
		})

		// Access control (admin only)
		r.Route("/manage/access", func(r chi.Router) {
			r.Get("/", handler.HandleAccessPage)
			r.Post("/user", handler.HandleCreateUser)
			r.Post("/user/{id}", handler.HandleEditUser)
			r.Delete("/user/{id}", handler.HandleDeleteUser)
		})

		// API key management (any authenticated user, for their own keys)
		r.Get("/api-keys", handler.HandleAPIKeysPage)
		r.Post("/api-keys", handler.HandleCreateAPIKey)
		r.Delete("/api-keys/{id}", handler.HandleDeleteAPIKey)
	})

	// Unauthenticated navigator JSON endpoint
	r.Get("/assessment/{id}/navigator.json", handler.HandleNavigatorJSON)

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	log.Printf("PurpleOps starting on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var appConfig *Config

func main() {
	appConfig = LoadConfig()
	InitDB(appConfig)
	InitSessions(appConfig.SecretKey)
	InitTemplates()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Static files
	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// Auth routes (no auth required)
	r.Get("/login", HandleLogin)
	r.Post("/login", HandleLoginPost)
	r.Get("/logout", HandleLogout)

	// All authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(AuthRequired)

		// Home / index
		r.Get("/", HandleIndex)
		r.Get("/index", HandleIndex)

		// Password management
		r.Get("/password/change", HandlePasswordChange)
		r.Post("/password/change", HandlePasswordChangePost)
		r.Get("/password/changed", HandlePasswordChanged)

		// MFA routes
		r.Get("/mfa/register", HandleMFARegister)
		r.Post("/mfa/register", HandleMFARegisterPost)
		r.Get("/mfa/verify", HandleMFAVerify)
		r.Post("/mfa/verify", HandleMFAVerifyPost)

		// Assessment CRUD
		r.Post("/assessment", HandleNewAssessment)
		r.Route("/assessment/{id}", func(r chi.Router) {
			r.Use(UserAssignedAssessment)
			r.Get("/", HandleLoadAssessment)
			r.Post("/", HandleEditAssessment)
			r.Delete("/", HandleDeleteAssessment)

			// Assessment utils
			r.Post("/multi/{field}", HandleAssessmentMulti)
			r.Get("/navigator", HandleAssessmentNavigator)
			r.Get("/stats", HandleAssessmentStats)
			r.Get("/assessment_hexagons.svg", HandleAssessmentHexagons)

			// Assessment export
			r.Get("/export/{filetype}", HandleExportAssessment)
			r.Get("/export/campaign", HandleExportCampaign)
			r.Get("/export/templates", HandleExportTestcases)
			r.Post("/export/report", HandleExportReport)
			r.Get("/export/navigator", HandleExportNavigator)
			r.Get("/export/entire", HandleExportEntire)

			// Assessment import
			r.Post("/import/template", HandleImportTemplate)
			r.Post("/import/navigator", HandleImportNavigator)
			r.Post("/import/campaign", HandleImportCampaign)
		})

		// Assessment import entire (no assessment ID in URL)
		r.Post("/assessment/import/entire", HandleImportEntire)

		// Navigator JSON (unauthenticated - handled separately)
		// Testcase routes
		r.Route("/testcase/{id}", func(r chi.Router) {
			r.Use(UserAssignedAssessment)
			r.Get("/", HandleLoadTestCase)
			r.Post("/", HandleSaveTestCase)
			r.Post("/single", HandleNewTestCase)
			r.Get("/toggle-visibility", HandleToggleVisibility)
			r.Get("/clone", HandleCloneTestCase)
			r.Get("/delete", HandleDeleteTestCase)
			r.Delete("/evidence/{colour}/{file}", HandleDeleteEvidence)
			r.Get("/evidence/{file}", HandleFetchEvidence)
		})

		// Access control (admin only)
		r.Route("/manage/access", func(r chi.Router) {
			r.Get("/", HandleAccessPage)
			r.Post("/user", HandleCreateUser)
			r.Post("/user/{id}", HandleEditUser)
			r.Delete("/user/{id}", HandleDeleteUser)
		})
	})

	// Unauthenticated navigator JSON endpoint
	r.Get("/assessment/{id}/navigator.json", HandleNavigatorJSON)

	addr := fmt.Sprintf("%s:%s", appConfig.Host, appConfig.Port)
	log.Printf("PurpleOps starting on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

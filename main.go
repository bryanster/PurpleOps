package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/config"
	"github.com/bryanster/purpleops/internal/db"
	"github.com/bryanster/purpleops/internal/handler"
	secmw "github.com/bryanster/purpleops/internal/middleware"
	"github.com/bryanster/purpleops/internal/render"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()
	db.InitDB(cfg)
	ssoEnabled := cfg.OAuthEnabled || cfg.SAMLEnabled
	sessionMiddleware := auth.InitSessions(cfg.SecretKey, ssoEnabled, cfg.Debug)
	render.InitTemplates(cfg.Debug)

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

	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(secmw.SecurityHeaders)
	r.Use(sessionMiddleware)

	r.Static("/static", "./static")

	// CSRF-exempt routes: external callbacks and public API.
	r.POST("/auth/saml/acs", handler.HandleSAMLACS)
	r.GET("/assessment/:id/navigator.json", handler.HandleNavigatorJSON)

	unauth := r.Group("/")
	{
		// Auth routes (no auth required).
		unauth.GET("/login", handler.HandleLogin)
		unauth.POST("/login", rateLimitByIP(10, time.Minute), handler.HandleLoginPost)
		unauth.GET("/logout", handler.HandleLogout)

		// OAuth SSO routes.
		unauth.GET("/auth/oauth/login", handler.HandleOAuthLogin)
		unauth.GET("/auth/oauth/callback", handler.HandleOAuthCallback)

		// SAML SSO routes (login + metadata only; ACS is exempt above).
		unauth.GET("/auth/saml/login", handler.HandleSAMLLogin)
		unauth.GET("/auth/saml/metadata", handler.HandleSAMLMetadata)

		// All authenticated routes.
		authed := unauth.Group("/")
		authed.Use(auth.AuthRequired)
		{
			// Home / index.
			authed.GET("/", handler.HandleIndex)
			authed.GET("/index", handler.HandleIndex)

			// Password management.
			authed.GET("/password/change", handler.HandlePasswordChange)
			authed.POST("/password/change", rateLimitByIP(10, time.Minute), handler.HandlePasswordChangePost)
			authed.GET("/password/changed", handler.HandlePasswordChanged)

			// MFA routes.
			authed.GET("/mfa/register", handler.HandleMFARegister)
			authed.POST("/mfa/register", handler.HandleMFARegisterPost)
			authed.GET("/mfa/verify", handler.HandleMFAVerify)
			authed.POST("/mfa/verify", rateLimitByIP(10, time.Minute), handler.HandleMFAVerifyPost)

			// Assessment import entire (no assessment ID in URL; register before :id group).
			authed.POST("/assessment/import/entire", handler.HandleImportEntire)

			// Assessment CRUD.
			authed.POST("/assessment", handler.HandleNewAssessment)

			assessmentGroup := authed.Group("/assessment/:id")
			assessmentGroup.Use(auth.UserAssignedAssessment)
			{
				assessmentGroup.GET("/", handler.HandleLoadAssessment)
				assessmentGroup.POST("/", handler.HandleEditAssessment)
				assessmentGroup.DELETE("/", handler.HandleDeleteAssessment)

				// Assessment utils.
				assessmentGroup.POST("/multi/:field", handler.HandleAssessmentMulti)
				assessmentGroup.GET("/navigator", handler.HandleAssessmentNavigator)
				assessmentGroup.GET("/stats", handler.HandleAssessmentStats)
				assessmentGroup.GET("/assessment_hexagons.svg", handler.HandleAssessmentHexagons)

				// Assessment export.
				assessmentGroup.GET("/export/:filetype", handler.HandleExportAssessment)
				assessmentGroup.GET("/export/campaign", handler.HandleExportCampaign)
				assessmentGroup.GET("/export/templates", handler.HandleExportTestcases)
				assessmentGroup.GET("/export/navigator", handler.HandleExportNavigator)
				assessmentGroup.GET("/export/entire", handler.HandleExportEntire)

				// Assessment import.
				assessmentGroup.POST("/import/template", handler.HandleImportTemplate)
				assessmentGroup.POST("/import/navigator", handler.HandleImportNavigator)
				assessmentGroup.POST("/import/campaign", handler.HandleImportCampaign)
			}

			// Testcase routes.
			testcaseGroup := authed.Group("/testcase/:id")
			testcaseGroup.Use(auth.UserAssignedAssessment)
			{
				testcaseGroup.GET("/", handler.HandleLoadTestCase)
				testcaseGroup.POST("/", handler.HandleSaveTestCase)
				testcaseGroup.POST("/single", handler.HandleNewTestCase)
				testcaseGroup.GET("/toggle-visibility", handler.HandleToggleVisibility)
				testcaseGroup.GET("/toggle-timer", handler.HandleToggleTimer)
				testcaseGroup.GET("/clone", handler.HandleCloneTestCase)
				testcaseGroup.GET("/delete", handler.HandleDeleteTestCase)
				testcaseGroup.DELETE("/evidence/:colour/:file", handler.HandleDeleteEvidence)
				testcaseGroup.GET("/evidence/:file", handler.HandleFetchEvidence)
			}

			// Access control (admin only).
			authed.GET("/manage/access", handler.HandleAccessPage)
			authed.POST("/manage/access/user", handler.HandleCreateUser)
			authed.POST("/manage/access/user/:id", handler.HandleEditUser)
			authed.DELETE("/manage/access/user/:id", handler.HandleDeleteUser)

			// API key management.
			authed.GET("/api-keys", handler.HandleAPIKeysPage)
			authed.POST("/api-keys", rateLimitByIP(10, time.Minute), handler.HandleCreateAPIKey)
			authed.DELETE("/api-keys/:id", handler.HandleDeleteAPIKey)
		}
	}

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	log.Printf("PurpleOps starting on %s", addr)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

// rateLimitByIP returns a gin.HandlerFunc that limits to n requests per window per IP.
// Uses a simple fixed-window counter.
func rateLimitByIP(n int, per time.Duration) gin.HandlerFunc {
	type window struct {
		count    int
		windowAt time.Time
	}
	var mu sync.Mutex
	windows := make(map[string]*window)

	// Cleanup goroutine: remove stale entries.
	go func() {
		for range time.Tick(per) {
			mu.Lock()
			cutoff := time.Now().Add(-per)
			for ip, w := range windows {
				if w.windowAt.Before(cutoff) {
					delete(windows, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		w, ok := windows[ip]
		if !ok || now.Sub(w.windowAt) >= per {
			w = &window{windowAt: now}
			windows[ip] = w
		}
		w.count++
		count := w.count
		mu.Unlock()

		if count > n {
			c.String(http.StatusTooManyRequests, "Too many requests")
			c.Abort()
			return
		}
		c.Next()
	}
}

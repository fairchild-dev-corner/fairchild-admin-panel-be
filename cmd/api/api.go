package core

import (
	"database/sql"
	"fmt"
	"log"

	applications_handler "fairchild-admin-panel-be/internal/handlers/applications"
	audit_handler "fairchild-admin-panel-be/internal/handlers/audit"
	auth_handler "fairchild-admin-panel-be/internal/handlers/auth"
	dashboard_handler "fairchild-admin-panel-be/internal/handlers/dashboard"
	health_handler "fairchild-admin-panel-be/internal/handlers/health"
	members_handler "fairchild-admin-panel-be/internal/handlers/members"
	metrics_handler "fairchild-admin-panel-be/internal/handlers/metrics"
	portal_handler "fairchild-admin-panel-be/internal/handlers/portal"
	site_handler "fairchild-admin-panel-be/internal/handlers/site"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/config"
	applications_repo "fairchild-admin-panel-be/internal/repositories/applications"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	auth_repo "fairchild-admin-panel-be/internal/repositories/auth"
	dashboard_repo "fairchild-admin-panel-be/internal/repositories/dashboard"
	members_repo "fairchild-admin-panel-be/internal/repositories/members"
	metrics_repo "fairchild-admin-panel-be/internal/repositories/metrics"
	portal_repo "fairchild-admin-panel-be/internal/repositories/portal"
	site_repo "fairchild-admin-panel-be/internal/repositories/site"
	router "fairchild-admin-panel-be/internal/routes"
	applications_service "fairchild-admin-panel-be/internal/services/applications"
	audit_service "fairchild-admin-panel-be/internal/services/audit"
	auth_service "fairchild-admin-panel-be/internal/services/auth"
	dashboard_service "fairchild-admin-panel-be/internal/services/dashboard"
	health_service "fairchild-admin-panel-be/internal/services/health"
	members_service "fairchild-admin-panel-be/internal/services/members"
	metrics_service "fairchild-admin-panel-be/internal/services/metrics"
	portal_service "fairchild-admin-panel-be/internal/services/portal"
	site_service "fairchild-admin-panel-be/internal/services/site"
	"fairchild-admin-panel-be/pkg/database"

	"github.com/gin-gonic/gin"
)

type APIServer struct {
	server        *gin.Engine
	hostConf      *models.HostConfig
	mysqlDB       *sql.DB
	branchDBs     database.BranchDBs
	jwtConf       *models.JWTConfig
	healthTargets []models.HealthCheckTarget
}

func NewAPIServer(
	s *gin.Engine,
	h *models.HostConfig,
	mysqlDB *sql.DB,
	branchDBs database.BranchDBs,
	jwtConf *models.JWTConfig,
	healthTargets []models.HealthCheckTarget,
) *APIServer {
	return &APIServer{
		server:        s,
		hostConf:      h,
		mysqlDB:       mysqlDB,
		branchDBs:     branchDBs,
		jwtConf:       jwtConf,
		healthTargets: healthTargets,
	}
}

func (s *APIServer) Run() {

	// Define Routes Config
	routes := router.NewRouterConfig(s.server, s.hostConf.AllowedOrigins)
	routes.InitConfig()
	api := routes.RoutesGroup()

	// auth: repo -> service -> handler (username + password, no OTP)
	authRepo := auth_repo.NewAuthRepository(s.mysqlDB)
	tokenService := auth_service.NewTokenService(s.jwtConf)
	authService, err := auth_service.NewAuthService(authRepo, tokenService)
	if err != nil {
		log.Fatalf("unable to init auth service: %v", err)
	}
	authHandler := auth_handler.NewHandler(authService, s.hostConf.BuildEnv)
	authHandler.RegisterRoutes(api)

	// admin: every route below sits behind RequirePanelAuth, which accepts
	// only active users_panel accounts (admin or staff). Admin-only routes
	// add middlewares.RequireRole(cc.ROLE_ADMIN) on top.
	admin := api.Group("/admin", middlewares.RequirePanelAuth(authService))

	// dashboard: repo -> service -> handler
	dashboardRepo := dashboard_repo.NewDashboardRepository(s.mysqlDB, s.branchDBs)
	dashboardService := dashboard_service.NewDashboardService(dashboardRepo)
	dashboard_handler.NewHandler(dashboardService).RegisterRoutes(admin)

	// applications: repo -> service -> handler (loan + membership reviews)
	applicationsRepo := applications_repo.NewApplicationsRepository(s.mysqlDB)
	applicationsService := applications_service.NewApplicationsService(applicationsRepo)
	applications_handler.NewHandler(applicationsService).RegisterRoutes(admin)

	// members: repos -> service -> handler (admin_members CRUD + read-only
	// branch ledger members)
	membersRepo := members_repo.NewMembersRepository(s.mysqlDB)
	ledgerMembersRepo := members_repo.NewLedgerMembersRepository(s.branchDBs)
	membersService := members_service.NewMembersService(membersRepo, ledgerMembersRepo)
	members_handler.NewHandler(membersService).RegisterRoutes(admin)

	// portal: repo -> service -> handler (fairchild_be `users`: list, contact
	// update, remove; ledger details looked up per page)
	portalRepo := portal_repo.NewPortalRepository(s.mysqlDB)
	portalService := portal_service.NewPortalService(portalRepo, ledgerMembersRepo)
	portal_handler.NewHandler(portalService).RegisterRoutes(admin)

	// site: repo -> service -> handler (maintenance mode, admin + public)
	siteRepo := site_repo.NewSiteRepository(s.mysqlDB)
	siteService := site_service.NewSiteService(siteRepo)
	siteHandler := site_handler.NewHandler(siteService)
	siteHandler.RegisterRoutes(admin)
	siteHandler.RegisterPublicRoutes(api)

	// health: service -> handler (pings DBs and HEALTH_CHECK_URLS)
	healthService := health_service.NewHealthService(s.mysqlDB, s.branchDBs, s.healthTargets)
	healthHandler := health_handler.NewHandler(healthService)
	healthHandler.RegisterRoutes(admin)
	healthHandler.RegisterPublicRoutes(api)

	// metrics: repo -> service -> handler
	metricsRepo := metrics_repo.NewMetricsRepository(s.mysqlDB)
	metricsService := metrics_service.NewMetricsService(metricsRepo)
	metrics_handler.NewHandler(metricsService).RegisterRoutes(admin)

	// audit: repo -> service -> handler (writes happen inside each mutation)
	auditRepo := audit_repo.NewAuditRepository(s.mysqlDB)
	auditService := audit_service.NewAuditService(auditRepo)
	audit_handler.NewHandler(auditService).RegisterRoutes(admin)

	addr := fmt.Sprintf(":%s", s.hostConf.Port)
	if err := s.server.Run(addr); err != nil {
		log.Fatalf("unable to serve: %v", err)
	}
}

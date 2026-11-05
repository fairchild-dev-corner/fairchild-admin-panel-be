package core

import (
	"database/sql"
	"fmt"
	"log"

	applications_handler "fairchild-admin-panel-be/internal/handlers/applications"
	audit_handler "fairchild-admin-panel-be/internal/handlers/audit"
	auth_handler "fairchild-admin-panel-be/internal/handlers/auth"
	blasts_handler "fairchild-admin-panel-be/internal/handlers/blasts"
	claims_handler "fairchild-admin-panel-be/internal/handlers/claims"
	contact_handler "fairchild-admin-panel-be/internal/handlers/contact"
	dashboard_handler "fairchild-admin-panel-be/internal/handlers/dashboard"
	elections_handler "fairchild-admin-panel-be/internal/handlers/elections"
	health_handler "fairchild-admin-panel-be/internal/handlers/health"
	locks_handler "fairchild-admin-panel-be/internal/handlers/locks"
	members_handler "fairchild-admin-panel-be/internal/handlers/members"
	metrics_handler "fairchild-admin-panel-be/internal/handlers/metrics"
	portal_handler "fairchild-admin-panel-be/internal/handlers/portal"
	site_handler "fairchild-admin-panel-be/internal/handlers/site"
	voters_handler "fairchild-admin-panel-be/internal/handlers/voters"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/config"
	applications_repo "fairchild-admin-panel-be/internal/repositories/applications"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	auth_repo "fairchild-admin-panel-be/internal/repositories/auth"
	blasts_repo "fairchild-admin-panel-be/internal/repositories/blasts"
	claims_repo "fairchild-admin-panel-be/internal/repositories/claims"
	contact_repo "fairchild-admin-panel-be/internal/repositories/contact"
	dashboard_repo "fairchild-admin-panel-be/internal/repositories/dashboard"
	elections_repo "fairchild-admin-panel-be/internal/repositories/elections"
	locks_repo "fairchild-admin-panel-be/internal/repositories/locks"
	members_repo "fairchild-admin-panel-be/internal/repositories/members"
	metrics_repo "fairchild-admin-panel-be/internal/repositories/metrics"
	portal_repo "fairchild-admin-panel-be/internal/repositories/portal"
	site_repo "fairchild-admin-panel-be/internal/repositories/site"
	voters_repo "fairchild-admin-panel-be/internal/repositories/voters"
	router "fairchild-admin-panel-be/internal/routes"
	applications_service "fairchild-admin-panel-be/internal/services/applications"
	audit_service "fairchild-admin-panel-be/internal/services/audit"
	auth_service "fairchild-admin-panel-be/internal/services/auth"
	blasts_service "fairchild-admin-panel-be/internal/services/blasts"
	claims_service "fairchild-admin-panel-be/internal/services/claims"
	contact_service "fairchild-admin-panel-be/internal/services/contact"
	dashboard_service "fairchild-admin-panel-be/internal/services/dashboard"
	elections_service "fairchild-admin-panel-be/internal/services/elections"
	health_service "fairchild-admin-panel-be/internal/services/health"
	locks_service "fairchild-admin-panel-be/internal/services/locks"
	members_service "fairchild-admin-panel-be/internal/services/members"
	"fairchild-admin-panel-be/internal/services/messaging"
	metrics_service "fairchild-admin-panel-be/internal/services/metrics"
	portal_service "fairchild-admin-panel-be/internal/services/portal"
	site_service "fairchild-admin-panel-be/internal/services/site"
	voters_service "fairchild-admin-panel-be/internal/services/voters"
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
	messagingConf *models.MessagingConfig
}

func NewAPIServer(
	s *gin.Engine,
	h *models.HostConfig,
	mysqlDB *sql.DB,
	branchDBs database.BranchDBs,
	jwtConf *models.JWTConfig,
	healthTargets []models.HealthCheckTarget,
	messagingConf *models.MessagingConfig,
) *APIServer {
	return &APIServer{
		server:        s,
		hostConf:      h,
		mysqlDB:       mysqlDB,
		branchDBs:     branchDBs,
		jwtConf:       jwtConf,
		healthTargets: healthTargets,
		messagingConf: messagingConf,
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

	// Email (SMTP) and text (Movider) for messages to members; nil when that
	// provider isn't configured.
	var emailSender messaging.EmailSender
	var smsSender messaging.SMSSender
	if s.messagingConf.SMTP != nil {
		emailSender = messaging.NewSMTPClient(s.messagingConf.SMTP)
	}
	if s.messagingConf.Movider != nil {
		smsSender = messaging.NewMoviderClient(s.messagingConf.Movider)
	}

	// account claims: repo -> service -> handler (fairchild_be
	// `account_claim_requests`: list, approve = copy the requested
	// email/mobile onto the matching portal account + tell the member by
	// email and text, reject)
	claimsRepo := claims_repo.NewClaimsRepository(s.mysqlDB)
	claimsService := claims_service.NewClaimsService(claimsRepo, emailSender, smsSender)
	claims_handler.NewHandler(claimsService).RegisterRoutes(admin)

	// locked accounts: repo -> service -> handler (fairchild_be
	// `locked_member_accounts`: list locked/unlocked, unlock + tell the
	// member by email and text)
	locksRepo := locks_repo.NewLocksRepository(s.mysqlDB)
	locksService := locks_service.NewLocksService(locksRepo, emailSender, smsSender)
	locks_handler.NewHandler(locksService).RegisterRoutes(admin)

	// contact messages: repo -> service -> handler (fairchild_be
	// `contact_messages`, read only; replies by email (SMTP) or text
	// (Movider), recorded in `contact_message_replies`)
	contactRepo := contact_repo.NewContactRepository(s.mysqlDB)
	contactService := contact_service.NewContactService(contactRepo, emailSender, smsSender)
	contact_handler.NewHandler(contactService).RegisterRoutes(admin)

	// SMS/email blasts: repo -> service -> handler (`message_blasts` +
	// `message_blast_recipients`; audience from fairchild_be `users`, branch
	// via the portal ledger index; a background worker sends them)
	blastsRepo := blasts_repo.NewBlastsRepository(s.mysqlDB)
	blastsService := blasts_service.NewBlastsService(blastsRepo, portalService, emailSender, smsSender, s.messagingConf.Blast)
	blasts_handler.NewHandler(blastsService).RegisterRoutes(admin)

	// elections: repo -> service -> handler (`elections` + positions +
	// candidates; results from `election_votes`, which the member portal
	// writes; candidates and voters from fairchild_be `users`)
	electionsRepo := elections_repo.NewElectionsRepository(s.mysqlDB)
	electionsService := elections_service.NewElectionsService(electionsRepo)
	elections_handler.NewHandler(electionsService).RegisterRoutes(admin)

	// voters: repo -> service -> handler (`voter_master_list`, uploaded from
	// Excel; Members List = fairchild_be `users` joined with it)
	votersRepo := voters_repo.NewVotersRepository(s.mysqlDB)
	votersService := voters_service.NewVotersService(votersRepo)
	voters_handler.NewHandler(votersService).RegisterRoutes(admin)

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

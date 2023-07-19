package api

import (
	"fmt"
	db "github.com/aalug/go-gin-job-search/db/sqlc"
	"github.com/aalug/go-gin-job-search/token"
	"github.com/aalug/go-gin-job-search/utils"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Server serves HTTP  requests for the service
type Server struct {
	config     utils.Config
	store      db.Store
	tokenMaker token.Maker
	router     *gin.Engine
}

// NewServer creates a new HTTP server and setups routing
func NewServer(config utils.Config, store db.Store) (*Server, error) {
	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create token maker: %w", err)
	}
	server := &Server{
		config:     config,
		store:      store,
		tokenMaker: tokenMaker,
	}

	server.setupRouter()

	return server, nil
}

// setupRouter sets up the HTTP routing
func (server *Server) setupRouter() {
	router := gin.Default()

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowHeaders = append(corsConfig.AllowHeaders, "Authorization")
	router.Use(cors.New(corsConfig))

	// === users ===
	router.POST("/users", server.createUser)
	router.POST("/users/login", server.loginUser)

	// === employers ===
	router.POST("/employers", server.createEmployer)
	router.POST("/employers/login", server.loginEmployer)

	// === jobs ===
	router.GET("/jobs/:id", server.getJob)

	// ===== routes that require authentication =====
	// === users ===
	authRoutes := router.Group("/").Use(authMiddleware(server.tokenMaker))
	authRoutes.PATCH("/users", server.updateUser)
	authRoutes.PATCH("/users/password", server.updateUserPassword)
	authRoutes.DELETE("/users", server.deleteUser)
	authRoutes.GET("/users", server.getUser)

	// employers
	authRoutes.GET("/employers", server.getEmployer)
	authRoutes.PATCH("/employers", server.updateEmployer)
	authRoutes.PATCH("/employers/password", server.updateEmployerPassword)
	authRoutes.DELETE("/employers", server.deleteEmployer)

	// jobs
	authRoutes.POST("/jobs", server.createJob)
	authRoutes.DELETE("/jobs/:id", server.deleteJob)

	server.router = router
}

// Start runs the HTTP server on a given address
func (server *Server) Start(address string) error {
	return server.router.Run(address)
}

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}

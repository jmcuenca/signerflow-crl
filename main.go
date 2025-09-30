package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"signerflow-crl/cache"
	"signerflow-crl/config"
	"signerflow-crl/database"
	"signerflow-crl/handlers"
	"signerflow-crl/scheduler"
	"signerflow-crl/services"
)

func main() {
	cfg := config.LoadConfig()

	db, err := database.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Error conectando a PostgreSQL: %v", err)
	}
	defer db.Close()

	var redisClient *cache.RedisClient
	if cfg.RedisURL != "" {
		redisClient, err = cache.NewRedisClient(cfg.RedisURL, cfg.RedisPassword, cfg.RedisDB)
		if err != nil {
			log.Printf("Warning: Error conectando a Redis: %v", err)
			log.Println("Continuando sin cache Redis")
		}
		if redisClient != nil {
			defer redisClient.Close()
		}
	}

	crlService := services.NewCRLService(db, redisClient)

	crlScheduler := scheduler.NewScheduler(crlService, cfg.CRLURLsFile)
	err = crlScheduler.Start()
	if err != nil {
		log.Fatalf("Error iniciando scheduler: %v", err)
	}
	defer crlScheduler.Stop()

	certificateHandler := handlers.NewCertificateHandler(crlService, db, redisClient)

	router := setupRouter(certificateHandler)

	go func() {
		log.Printf("Servidor iniciado en puerto %s", cfg.Port)
		if err := router.Run(":" + cfg.Port); err != nil {
			log.Fatalf("Error iniciando servidor: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Cerrando servidor...")
}

func setupRouter(handler *handlers.CertificateHandler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	v1 := router.Group("/api/v1")
	{
		v1.GET("/health", handler.GetHealth)
		v1.GET("/stats", handler.GetStats)

		certificates := v1.Group("/certificates")
		{
			certificates.GET("/check/:serial", handler.CheckCertificate)
			certificates.GET("/valid/:serial", handler.ValidCertificate)
			certificates.GET("/details/:serial", handler.GetCertificateDetails)
		}

		admin := v1.Group("/admin")
		{
			admin.POST("/refresh", handler.ForceRefresh)
		}
	}

	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"service":     "SignerFlow CRL Service",
			"version":     "1.0.0",
			"description": "Servicio de verificación de certificados revocados",
			"endpoints": gin.H{
				"health":              "/api/v1/health",
				"stats":               "/api/v1/stats",
				"check_certificate":   "/api/v1/certificates/check/:serial",
				"certificate_details": "/api/v1/certificates/details/:serial",
				"force_refresh":       "/api/v1/admin/refresh",
			},
		})
	})

	return router
}

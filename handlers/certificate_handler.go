package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/signerflow/crl-service/cache"
	"github.com/signerflow/crl-service/database"
	"github.com/signerflow/crl-service/services"
)

type CertificateHandler struct {
	crlService *services.CRLService
	db         *database.DB
	redis      *cache.RedisClient
}

func NewCertificateHandler(crlService *services.CRLService, db *database.DB, redis *cache.RedisClient) *CertificateHandler {
	return &CertificateHandler{
		crlService: crlService,
		db:         db,
		redis:      redis,
	}
}

func (h *CertificateHandler) CheckCertificate(c *gin.Context) {
	serial := c.Param("serial")
	if serial == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Serial requerido",
			"message": "Debe proporcionar el número de serie del certificado",
		})
		return
	}

	serial = strings.ToUpper(strings.TrimSpace(serial))

	if h.redis != nil {
		h.redis.IncrementStats("stats:requests_total")
	}

	status, err := h.crlService.CheckCertificateStatus(serial)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Error interno del servidor",
			"message": "Error al verificar el estado del certificado",
		})
		return
	}

	c.JSON(http.StatusOK, status)
}
func (h *CertificateHandler) ValidCertificate(c *gin.Context) {
	serial := c.Param("serial")
	if serial == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Serial requerido",
			"message": "Debe proporcionar el número de serie del certificado",
		})
		return
	}

	serial = strings.ToUpper(strings.TrimSpace(serial))

	if h.redis != nil {
		h.redis.IncrementStats("stats:requests_total")
	}

	status, err := h.crlService.CheckCertificateStatus(serial)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Error interno del servidor",
			"message": "Error al verificar el estado del certificado",
		})
		return
	}
	if status.IsRevoked {
		c.String(http.StatusOK, status.RevocationDate.Format(time.RFC3339))
	} else {
		c.String(http.StatusOK, "")
	}

}

func (h *CertificateHandler) GetHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "signerflow-crl-service",
		"version": "1.0.0",
	})
}

func (h *CertificateHandler) GetStats(c *gin.Context) {
	dbStats, err := h.db.GetCRLStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error obteniendo estadísticas de base de datos",
		})
		return
	}

	response := gin.H{
		"database": dbStats,
	}

	if h.redis != nil {
		redisStats, err := h.redis.GetStats()
		if err != nil {
			response["cache"] = gin.H{"error": "Error obteniendo estadísticas de cache"}
		} else {
			response["cache"] = redisStats
		}
	}

	c.JSON(http.StatusOK, response)
}

func (h *CertificateHandler) ForceRefresh(c *gin.Context) {
	crlURLsFile := c.Query("file")
	if crlURLsFile == "" {
		crlURLsFile = "crl_urls.json"
	}

	go func() {
		err := h.crlService.ProcessAllCRLs(crlURLsFile)
		if err != nil {
			// Log error but don't block the response
			// In a production environment, you might want to use proper logging
			println("Error en procesamiento manual de CRLs:", err.Error())
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Actualización de CRLs iniciada en segundo plano",
		"status":  "processing",
	})
}

func (h *CertificateHandler) GetCertificateDetails(c *gin.Context) {
	serial := c.Param("serial")
	if serial == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Serial requerido",
			"message": "Debe proporcionar el número de serie del certificado",
		})
		return
	}

	serial = strings.ToUpper(strings.TrimSpace(serial))

	status, err := h.db.GetCertificateStatus(serial)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Error interno del servidor",
			"message": "Error al obtener detalles del certificado",
		})
		return
	}

	if !status.IsRevoked {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Certificado no encontrado",
			"message": "El certificado no está en la lista de revocación",
			"serial":  serial,
		})
		return
	}

	c.JSON(http.StatusOK, status)
}

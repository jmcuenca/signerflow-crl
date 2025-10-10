package services

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"signerflow-crl/cache"
	"signerflow-crl/database"
	"signerflow-crl/models"
)

type CRLService struct {
	db         *database.DB
	redis      *cache.RedisClient
	httpClient *http.Client
}

func NewCRLService(db *database.DB, redis *cache.RedisClient) *CRLService {
	// Crear HTTP client optimizado con pool de conexiones reutilizables
	transport := &http.Transport{
		MaxIdleConns:        100,              // Máximo de conexiones idle totales
		MaxIdleConnsPerHost: 20,               // Máximo de conexiones idle por host
		MaxConnsPerHost:     50,               // Máximo de conexiones por host
		IdleConnTimeout:     90 * time.Second, // Timeout para conexiones idle
		DisableCompression:  false,            // Habilitar compresión
		DisableKeepAlives:   false,            // Mantener conexiones vivas
	}

	return &CRLService{
		db:    db,
		redis: redis,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

func (s *CRLService) LoadCRLURLs(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening CRL URLs file: %v", err)
	}
	defer file.Close()

	var urls []string
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&urls)
	if err != nil {
		return nil, fmt.Errorf("error decoding CRL URLs JSON: %v", err)
	}

	return urls, nil
}

func (s *CRLService) ProcessAllCRLs(crlURLsFile string) error {
	urls, err := s.LoadCRLURLs(crlURLsFile)
	if err != nil {
		return fmt.Errorf("error loading CRL URLs: %v", err)
	}

	log.Printf("Starting to process %d CRL URLs", len(urls))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5)

	for _, crlURL := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			err := s.ProcessSingleCRL(url)
			if err != nil {
				log.Printf("Error processing CRL %s: %v", url, err)
			}
		}(crlURL)
	}

	wg.Wait()
	log.Printf("Finished processing all CRLs")

	if s.redis != nil {
		s.redis.IncrementStats("stats:crls_processed")
	}

	return nil
}

func (s *CRLService) ProcessSingleCRL(crlURL string) error {
	if s.redis != nil {
		processing, err := s.redis.IsCRLProcessing(crlURL)
		if err != nil {
			log.Printf("Error checking CRL processing status: %v", err)
		} else if processing {
			log.Printf("CRL %s is already being processed, skipping", crlURL)
			return nil
		}

		err = s.redis.SetCRLProcessing(crlURL, true)
		if err != nil {
			log.Printf("Error setting CRL processing status: %v", err)
		}
		defer s.redis.SetCRLProcessing(crlURL, false)
	}

	log.Printf("Processing CRL: %s", crlURL)

	crlData, err := s.downloadCRL(crlURL)
	if err != nil {
		return fmt.Errorf("error downloading CRL: %v", err)
	}

	crl, err := x509.ParseCRL(crlData)
	if err != nil {
		return fmt.Errorf("error parsing CRL: %v", err)
	}

	var issuerName pkix.Name
	issuerName.FillFromRDNSequence(&crl.TBSCertList.Issuer)
	issuerNameStr := s.extractIssuerName(issuerName)

	crlInfo := &models.CRLInfo{
		URL:           crlURL,
		Issuer:        issuerNameStr,
		NextUpdate:    crl.TBSCertList.NextUpdate,
		LastProcessed: time.Now(),
		CertCount:     len(crl.TBSCertList.RevokedCertificates),
	}

	err = s.db.InsertCRLInfo(crlInfo)
	if err != nil {
		log.Printf("Error inserting CRL info: %v", err)
	}

	// Procesar certificados en batch para mejor rendimiento
	batchSize := 500
	certificates := make([]*models.RevokedCertificate, 0, batchSize)

	processed := 0
	for _, revokedCert := range crl.TBSCertList.RevokedCertificates {
		serial := s.formatSerial(revokedCert.SerialNumber)

		reason := 0
		reasonText := ""

		for _, ext := range revokedCert.Extensions {
			if ext.Id.Equal([]int{2, 5, 29, 21}) {
				if len(ext.Value) > 0 {
					reason = int(ext.Value[0])
					if reasonText, exists := models.RevocationReasons[reason]; exists {
						reasonText = reasonText
					}
				}
			}
		}

		revokedCertificate := &models.RevokedCertificate{
			Serial:               serial,
			RevocationDate:       revokedCert.RevocationTime,
			Reason:               reason,
			ReasonText:           reasonText,
			CertificateAuthority: issuerNameStr,
		}

		certificates = append(certificates, revokedCertificate)

		// Insertar en batch cuando se alcanza el tamaño del batch
		if len(certificates) >= batchSize {
			err = s.db.BatchInsertRevokedCertificates(certificates)
			if err != nil {
				log.Printf("Error batch inserting certificates: %v", err)
			} else {
				processed += len(certificates)
			}

			// Cachear certificados en Redis
			if s.redis != nil {
				for _, cert := range certificates {
					status := &models.CertificateStatus{
						Serial:               cert.Serial,
						IsRevoked:            true,
						RevocationDate:       &cert.RevocationDate,
						Reason:               &cert.ReasonText,
						CertificateAuthority: &issuerNameStr,
					}
					err = s.redis.SetCertificateStatus(cert.Serial, status, 24*time.Hour)
					if err != nil {
						log.Printf("Error caching certificate status for %s: %v", cert.Serial, err)
					}
				}
			}

			certificates = make([]*models.RevokedCertificate, 0, batchSize)
		}
	}

	// Insertar certificados restantes
	if len(certificates) > 0 {
		err = s.db.BatchInsertRevokedCertificates(certificates)
		if err != nil {
			log.Printf("Error batch inserting remaining certificates: %v", err)
		} else {
			processed += len(certificates)
		}

		// Cachear certificados restantes en Redis
		if s.redis != nil {
			for _, cert := range certificates {
				status := &models.CertificateStatus{
					Serial:               cert.Serial,
					IsRevoked:            true,
					RevocationDate:       &cert.RevocationDate,
					Reason:               &cert.ReasonText,
					CertificateAuthority: &issuerNameStr,
				}
				err = s.redis.SetCertificateStatus(cert.Serial, status, 24*time.Hour)
				if err != nil {
					log.Printf("Error caching certificate status for %s: %v", cert.Serial, err)
				}
			}
		}
	}

	log.Printf("Successfully processed CRL %s: %d certificates processed", crlURL, processed)
	return nil
}

func (s *CRLService) downloadCRL(crlURL string) ([]byte, error) {
	parsedURL, err := url.Parse(crlURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	// Usar el cliente HTTP reutilizable con pool de conexiones
	req, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("User-Agent", "SignerFlow-CRL-Service/1.0")
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error downloading CRL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	return data, nil
}

func (s *CRLService) extractIssuerName(issuer pkix.Name) string {
	if issuer.CommonName != "" {
		return issuer.CommonName
	}

	if len(issuer.Organization) > 0 {
		return issuer.Organization[0]
	}

	if len(issuer.OrganizationalUnit) > 0 {
		return issuer.OrganizationalUnit[0]
	}

	return issuer.String()
}

func (s *CRLService) formatSerial(serial *big.Int) string {
	return serial.String()
}

// normalizeSerial converts hexadecimal serial numbers to decimal
// If the input is already decimal, it returns as-is
func (s *CRLService) normalizeSerial(serial string) string {
	return serial
}

func (s *CRLService) CheckCertificateStatus(serial string) (*models.CertificateStatus, error) {
	// Normalize serial to decimal format
	serial = s.normalizeSerial(serial)
	if s.redis != nil {
		status, err := s.redis.GetCertificateStatus(serial)
		if err != nil {
			log.Printf("Error getting certificate status from cache: %v", err)
		} else if status != nil {
			s.redis.IncrementStats("stats:cache_hits")
			return status, nil
		}
		s.redis.IncrementStats("stats:cache_misses")
	}

	status, err := s.db.GetCertificateStatus(serial)
	if err != nil {
		return nil, fmt.Errorf("error getting certificate status from database: %v", err)
	}

	if s.redis != nil && status != nil {
		ttl := 24 * time.Hour
		if status.IsRevoked {
			ttl = 7 * 24 * time.Hour
		}

		err = s.redis.SetCertificateStatus(serial, status, ttl)
		if err != nil {
			log.Printf("Error caching certificate status: %v", err)
		}
	}

	return status, nil
}

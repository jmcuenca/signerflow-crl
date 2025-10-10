package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	"signerflow-crl/models"
)

type DB struct {
	*sql.DB
	// Prepared statements para mejor rendimiento
	stmtGetCertStatus   *sql.Stmt
	stmtInsertCert      *sql.Stmt
	stmtInsertCRLInfo   *sql.Stmt
	stmtGetTotalCerts   *sql.Stmt
	stmtGetTotalCRLs    *sql.Stmt
	stmtGetLastUpdate   *sql.Stmt
}

func NewPostgresDB(databaseURL string) (*DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %v", err)
	}

	// Optimización del pool de conexiones
	db.SetMaxOpenConns(25)                 // Máximo de conexiones abiertas
	db.SetMaxIdleConns(10)                 // Conexiones idle en el pool
	db.SetConnMaxLifetime(5 * time.Minute) // Tiempo de vida máximo de una conexión
	db.SetConnMaxIdleTime(2 * time.Minute) // Tiempo máximo que una conexión puede estar idle

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error pinging database: %v", err)
	}

	database := &DB{DB: db}
	if err := database.createTables(); err != nil {
		return nil, fmt.Errorf("error creating tables: %v", err)
	}

	// Preparar statements para mejor rendimiento
	if err := database.prepareStatements(); err != nil {
		return nil, fmt.Errorf("error preparing statements: %v", err)
	}

	log.Println("Connected to PostgreSQL database with optimized pool settings")
	return database, nil
}

func (db *DB) prepareStatements() error {
	var err error

	// Statement para obtener estado de certificado
	db.stmtGetCertStatus, err = db.Prepare(`
		SELECT serial, revocation_date, reason, reason_text, certificate_authority
		FROM revoked_certificates
		WHERE serial = $1
	`)
	if err != nil {
		return fmt.Errorf("error preparing stmtGetCertStatus: %v", err)
	}

	// Statement para insertar certificado revocado
	db.stmtInsertCert, err = db.Prepare(`
		INSERT INTO revoked_certificates
		(serial, revocation_date, reason, reason_text, certificate_authority, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (serial)
		DO UPDATE SET
			revocation_date = EXCLUDED.revocation_date,
			reason = EXCLUDED.reason,
			reason_text = EXCLUDED.reason_text,
			certificate_authority = EXCLUDED.certificate_authority,
			updated_at = EXCLUDED.updated_at
	`)
	if err != nil {
		return fmt.Errorf("error preparing stmtInsertCert: %v", err)
	}

	// Statement para insertar CRL info
	db.stmtInsertCRLInfo, err = db.Prepare(`
		INSERT INTO crl_info
		(url, issuer, next_update, last_processed, cert_count, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (url)
		DO UPDATE SET
			issuer = EXCLUDED.issuer,
			next_update = EXCLUDED.next_update,
			last_processed = EXCLUDED.last_processed,
			cert_count = EXCLUDED.cert_count,
			updated_at = EXCLUDED.updated_at
	`)
	if err != nil {
		return fmt.Errorf("error preparing stmtInsertCRLInfo: %v", err)
	}

	// Statement para estadísticas
	db.stmtGetTotalCerts, err = db.Prepare("SELECT COUNT(*) FROM revoked_certificates")
	if err != nil {
		return fmt.Errorf("error preparing stmtGetTotalCerts: %v", err)
	}

	db.stmtGetTotalCRLs, err = db.Prepare("SELECT COUNT(*) FROM crl_info")
	if err != nil {
		return fmt.Errorf("error preparing stmtGetTotalCRLs: %v", err)
	}

	db.stmtGetLastUpdate, err = db.Prepare("SELECT COALESCE(MAX(last_processed), '1970-01-01') FROM crl_info")
	if err != nil {
		return fmt.Errorf("error preparing stmtGetLastUpdate: %v", err)
	}

	return nil
}

func (db *DB) createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS revoked_certificates (
		id SERIAL PRIMARY KEY,
		serial VARCHAR(255) NOT NULL UNIQUE,
		revocation_date TIMESTAMP NOT NULL,
		reason INTEGER NOT NULL DEFAULT 0,
		reason_text VARCHAR(255),
		certificate_authority VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_revoked_certificates_serial ON revoked_certificates(serial);
	CREATE INDEX IF NOT EXISTS idx_revoked_certificates_ca ON revoked_certificates(certificate_authority);
	CREATE INDEX IF NOT EXISTS idx_revoked_certificates_revocation_date ON revoked_certificates(revocation_date);
	CREATE INDEX IF NOT EXISTS idx_revoked_certificates_composite ON revoked_certificates(serial, certificate_authority);

	CREATE TABLE IF NOT EXISTS crl_info (
		id SERIAL PRIMARY KEY,
		url VARCHAR(500) NOT NULL UNIQUE,
		issuer VARCHAR(500) NOT NULL,
		next_update TIMESTAMP,
		last_processed TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		cert_count INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := db.Exec(query)
	return err
}

func (db *DB) InsertRevokedCertificate(cert *models.RevokedCertificate) error {
	// Usar prepared statement para mejor rendimiento
	_, err := db.stmtInsertCert.Exec(
		cert.Serial,
		cert.RevocationDate,
		cert.Reason,
		cert.ReasonText,
		cert.CertificateAuthority,
		time.Now(),
	)
	return err
}

// BatchInsertRevokedCertificates inserta múltiples certificados en una sola transacción
func (db *DB) BatchInsertRevokedCertificates(certs []*models.RevokedCertificate) error {
	if len(certs) == 0 {
		return nil
	}

	// Iniciar transacción
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Preparar statement dentro de la transacción
	stmt, err := tx.Prepare(`
		INSERT INTO revoked_certificates
		(serial, revocation_date, reason, reason_text, certificate_authority, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (serial)
		DO UPDATE SET
			revocation_date = EXCLUDED.revocation_date,
			reason = EXCLUDED.reason,
			reason_text = EXCLUDED.reason_text,
			certificate_authority = EXCLUDED.certificate_authority,
			updated_at = EXCLUDED.updated_at
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %v", err)
	}
	defer stmt.Close()

	// Insertar certificados en batch
	now := time.Now()
	for _, cert := range certs {
		_, err = stmt.Exec(
			cert.Serial,
			cert.RevocationDate,
			cert.Reason,
			cert.ReasonText,
			cert.CertificateAuthority,
			now,
		)
		if err != nil {
			return fmt.Errorf("error inserting certificate %s: %v", cert.Serial, err)
		}
	}

	// Confirmar transacción
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	return nil
}

func (db *DB) GetCertificateStatus(serial string) (*models.CertificateStatus, error) {
	// Usar prepared statement para mejor rendimiento
	var cert models.RevokedCertificate
	err := db.stmtGetCertStatus.QueryRow(serial).Scan(
		&cert.Serial,
		&cert.RevocationDate,
		&cert.Reason,
		&cert.ReasonText,
		&cert.CertificateAuthority,
	)

	if err == sql.ErrNoRows {
		return &models.CertificateStatus{
			Serial:    serial,
			IsRevoked: false,
		}, nil
	}

	if err != nil {
		return nil, err
	}

	reasonText := models.RevocationReasons[cert.Reason]
	if cert.ReasonText != "" {
		reasonText = cert.ReasonText
	}

	return &models.CertificateStatus{
		Serial:               serial,
		IsRevoked:           true,
		RevocationDate:      &cert.RevocationDate,
		Reason:              &reasonText,
		CertificateAuthority: &cert.CertificateAuthority,
	}, nil
}

func (db *DB) InsertCRLInfo(crlInfo *models.CRLInfo) error {
	// Usar prepared statement para mejor rendimiento
	_, err := db.stmtInsertCRLInfo.Exec(
		crlInfo.URL,
		crlInfo.Issuer,
		crlInfo.NextUpdate,
		crlInfo.LastProcessed,
		crlInfo.CertCount,
		time.Now(),
	)
	return err
}

func (db *DB) GetCRLStats() (map[string]interface{}, error) {
	var totalCerts int
	var totalCRLs int
	var lastUpdate time.Time

	// Usar prepared statements para mejor rendimiento
	err := db.stmtGetTotalCerts.QueryRow().Scan(&totalCerts)
	if err != nil {
		return nil, err
	}

	err = db.stmtGetTotalCRLs.QueryRow().Scan(&totalCRLs)
	if err != nil {
		return nil, err
	}

	err = db.stmtGetLastUpdate.QueryRow().Scan(&lastUpdate)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_revoked_certificates": totalCerts,
		"total_crls_processed":      totalCRLs,
		"last_update":               lastUpdate,
	}, nil
}

// Close cierra todas las prepared statements y la conexión a la base de datos
func (db *DB) Close() error {
	// Cerrar todos los prepared statements
	if db.stmtGetCertStatus != nil {
		db.stmtGetCertStatus.Close()
	}
	if db.stmtInsertCert != nil {
		db.stmtInsertCert.Close()
	}
	if db.stmtInsertCRLInfo != nil {
		db.stmtInsertCRLInfo.Close()
	}
	if db.stmtGetTotalCerts != nil {
		db.stmtGetTotalCerts.Close()
	}
	if db.stmtGetTotalCRLs != nil {
		db.stmtGetTotalCRLs.Close()
	}
	if db.stmtGetLastUpdate != nil {
		db.stmtGetLastUpdate.Close()
	}

	// Cerrar la conexión a la base de datos
	return db.DB.Close()
}
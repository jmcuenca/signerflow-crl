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
}

func NewPostgresDB(databaseURL string) (*DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error pinging database: %v", err)
	}

	database := &DB{db}
	if err := database.createTables(); err != nil {
		return nil, fmt.Errorf("error creating tables: %v", err)
	}

	log.Println("Connected to PostgreSQL database")
	return database, nil
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
	query := `
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
	`

	_, err := db.Exec(query,
		cert.Serial,
		cert.RevocationDate,
		cert.Reason,
		cert.ReasonText,
		cert.CertificateAuthority,
		time.Now(),
	)
	return err
}

func (db *DB) GetCertificateStatus(serial string) (*models.CertificateStatus, error) {
	query := `
	SELECT serial, revocation_date, reason, reason_text, certificate_authority
	FROM revoked_certificates
	WHERE serial = $1
	`

	var cert models.RevokedCertificate
	err := db.QueryRow(query, serial).Scan(
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
	query := `
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
	`

	_, err := db.Exec(query,
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

	err := db.QueryRow("SELECT COUNT(*) FROM revoked_certificates").Scan(&totalCerts)
	if err != nil {
		return nil, err
	}

	err = db.QueryRow("SELECT COUNT(*) FROM crl_info").Scan(&totalCRLs)
	if err != nil {
		return nil, err
	}

	err = db.QueryRow("SELECT COALESCE(MAX(last_processed), '1970-01-01') FROM crl_info").Scan(&lastUpdate)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_revoked_certificates": totalCerts,
		"total_crls_processed":      totalCRLs,
		"last_update":               lastUpdate,
	}, nil
}
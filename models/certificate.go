package models

import (
	"time"
)

type RevokedCertificate struct {
	ID                int       `json:"id" db:"id"`
	Serial            string    `json:"serial" db:"serial"`
	RevocationDate    time.Time `json:"revocation_date" db:"revocation_date"`
	Reason            int       `json:"reason" db:"reason"`
	ReasonText        string    `json:"reason_text" db:"reason_text"`
	CertificateAuthority string `json:"certificate_authority" db:"certificate_authority"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

type CertificateStatus struct {
	Serial     string    `json:"serial"`
	IsRevoked  bool      `json:"is_revoked"`
	RevocationDate *time.Time `json:"revocation_date,omitempty"`
	Reason     *string   `json:"reason,omitempty"`
	CertificateAuthority *string `json:"certificate_authority,omitempty"`
}

type CRLInfo struct {
	URL           string    `json:"url"`
	Issuer        string    `json:"issuer"`
	NextUpdate    time.Time `json:"next_update"`
	LastProcessed time.Time `json:"last_processed"`
	CertCount     int       `json:"cert_count"`
}

const (
	ReasonUnspecified          = 0
	ReasonKeyCompromise        = 1
	ReasonCACompromise         = 2
	ReasonAffiliationChanged   = 3
	ReasonSuperseded          = 4
	ReasonCessationOfOperation = 5
	ReasonCertificateHold      = 6
	ReasonRemoveFromCRL        = 8
	ReasonPrivilegeWithdrawn   = 9
	ReasonAACompromise         = 10
)

var RevocationReasons = map[int]string{
	ReasonUnspecified:          "No especificado",
	ReasonKeyCompromise:        "Compromiso de clave",
	ReasonCACompromise:         "Compromiso de CA",
	ReasonAffiliationChanged:   "Cambio de afiliación",
	ReasonSuperseded:          "Reemplazado",
	ReasonCessationOfOperation: "Cese de operaciones",
	ReasonCertificateHold:      "Retención de certificado",
	ReasonRemoveFromCRL:        "Eliminado de CRL",
	ReasonPrivilegeWithdrawn:   "Privilegio retirado",
	ReasonAACompromise:         "Compromiso de AA",
}
# SignerFlow CRL Service

Servicio REST en Go para verificar el estado de revocación de certificados digitales descargando y procesando Certificate Revocation Lists (CRLs) de Ecuador.

## Características

- ✅ **API REST** con endpoint GET `/api/v1/certificates/check/{serial}`
- ✅ **Descarga automática** de CRLs cada 10 minutos
- ✅ **Base de datos PostgreSQL** para almacenar certificados revocados
- ✅ **Cache Redis** para consultas rápidas
- ✅ **Procesamiento concurrente** de múltiples CRLs
- ✅ **Docker Compose** para fácil despliegue
- ✅ **Estadísticas y monitoreo** del servicio

## Estructura del Proyecto

```
.
├── cache/              # Cliente Redis
├── config/             # Configuración del servicio
├── database/           # Conexión y operaciones PostgreSQL
├── handlers/           # Controladores HTTP/REST
├── models/             # Modelos de datos
├── scheduler/          # Tareas programadas
├── services/           # Lógica de negocio CRL
├── main.go             # Punto de entrada
├── docker-compose.yml  # Servicios Docker
├── Dockerfile          # Imagen de la aplicación
├── crl_urls.json       # URLs de CRLs de Ecuador
└── .env                # Variables de entorno
```

## Instalación y Configuración

### 1. Prerrequisitos

- Go 1.21+
- Docker y Docker Compose
- PostgreSQL 15+ (si no usas Docker)
- Redis 7+ (si no usas Docker)

### 2. Configuración

Edita el archivo `.env` con tus configuraciones:

```env
PORT=8080
DATABASE_URL=postgres://crl_user:crl_password@localhost:5432/crl_db?sslmode=disable
REDIS_URL=localhost:6379
REDIS_PASSWORD=
CRL_URLS_FILE=crl_urls.json
```

### 3. Ejecutar con Docker (Recomendado)

```bash
# Iniciar servicios de base de datos
docker-compose up -d postgres redis

# Instalar dependencias Go
go mod tidy

# Ejecutar la aplicación
go run main.go
```

### 4. Ejecutar todo en Docker

```bash
# Descomenta la sección crl-service en docker-compose.yml
# Luego ejecuta:
docker-compose up -d
```

## API Endpoints

### Verificar Estado de Certificado
```http
GET /api/v1/certificates/check/{serial}
```

**Respuesta:**
```json
{
  "serial": "1234567890ABCDEF",
  "is_revoked": true,
  "revocation_date": "2024-01-15T10:30:00Z",
  "reason": "Compromiso de clave",
  "certificate_authority": "AUTORIDAD DE CERTIFICACION SUBCA-1 SECURITY DATA"
}
```

### Detalles del Certificado
```http
GET /api/v1/certificates/details/{serial}
```

### Estadísticas del Servicio
```http
GET /api/v1/stats
```

### Estado de Salud
```http
GET /api/v1/health
```

### Forzar Actualización
```http
POST /api/v1/admin/refresh
```

## Ejemplos de Uso

### cURL
```bash
# Verificar certificado
curl "http://localhost:8080/api/v1/certificates/check/1234567890ABCDEF"

# Obtener estadísticas
curl "http://localhost:8080/api/v1/stats"

# Forzar actualización
curl -X POST "http://localhost:8080/api/v1/admin/refresh"
```

### JavaScript/Fetch
```javascript
// Verificar estado de certificado
const response = await fetch('/api/v1/certificates/check/1234567890ABCDEF');
const status = await response.json();

if (status.is_revoked) {
    console.log('Certificado REVOCADO:', status.reason);
} else {
    console.log('Certificado VÁLIDO');
}
```

## Base de Datos

### Tabla: revoked_certificates
```sql
CREATE TABLE revoked_certificates (
    id SERIAL PRIMARY KEY,
    serial VARCHAR(255) NOT NULL UNIQUE,
    revocation_date TIMESTAMP NOT NULL,
    reason INTEGER NOT NULL DEFAULT 0,
    reason_text VARCHAR(255),
    certificate_authority VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Tabla: crl_info
```sql
CREATE TABLE crl_info (
    id SERIAL PRIMARY KEY,
    url VARCHAR(500) NOT NULL UNIQUE,
    issuer VARCHAR(500) NOT NULL,
    next_update TIMESTAMP,
    last_processed TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    cert_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## Monitoreo y Logs

El servicio proporciona logs detallados y métricas:

- **Logs de procesamiento CRL**
- **Estadísticas de cache Redis**
- **Métricas de base de datos**
- **Contador de requests HTTP**

## Arquitectura

```
HTTP Request → Gin Router → Handler → CRL Service
                                         ↓
                            Redis Cache ← → PostgreSQL
                                         ↓
                            Scheduler → CRL Download → Parser
```

## Seguridad

- Validación de entrada en todos los endpoints
- Headers CORS configurados
- Timeouts en descargas HTTP
- Usuario no-root en Docker
- Logs de auditoría

## Contribuir

1. Fork el proyecto
2. Crea una rama feature (`git checkout -b feature/nueva-funcionalidad`)
3. Commit tus cambios (`git commit -am 'Agregar nueva funcionalidad'`)
4. Push a la rama (`git push origin feature/nueva-funcionalidad`)
5. Crea un Pull Request

## Licencia

Este proyecto está bajo la Licencia MIT.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"signerflow-crl/models"
)

type RedisClient struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisClient(redisURL, password string, db int) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: password,
		DB:       db,
		// Optimización del pool de conexiones
		PoolSize:           20,              // Tamaño del pool de conexiones
		MinIdleConns:       5,               // Mínimo de conexiones idle
		MaxConnAge:         5 * time.Minute, // Edad máxima de una conexión
		PoolTimeout:        4 * time.Second, // Timeout para obtener conexión del pool
		IdleTimeout:        3 * time.Minute, // Tiempo antes de cerrar conexiones idle
		IdleCheckFrequency: 1 * time.Minute, // Frecuencia de chequeo de conexiones idle
		// Timeouts
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx := context.Background()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("error connecting to Redis: %v", err)
	}

	log.Println("Connected to Redis with optimized pool settings")
	return &RedisClient{
		client: rdb,
		ctx:    ctx,
	}, nil
}

func (r *RedisClient) SetCertificateStatus(serial string, status *models.CertificateStatus, ttl time.Duration) error {
	key := fmt.Sprintf("cert:%s", serial)

	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("error marshaling certificate status: %v", err)
	}

	err = r.client.Set(r.ctx, key, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("error setting certificate status in Redis: %v", err)
	}

	return nil
}

func (r *RedisClient) GetCertificateStatus(serial string) (*models.CertificateStatus, error) {
	key := fmt.Sprintf("cert:%s", serial)

	val, err := r.client.Get(r.ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error getting certificate status from Redis: %v", err)
	}

	var status models.CertificateStatus
	err = json.Unmarshal([]byte(val), &status)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling certificate status: %v", err)
	}

	return &status, nil
}

func (r *RedisClient) SetCRLProcessing(url string, processing bool) error {
	key := fmt.Sprintf("crl_processing:%s", url)

	var value string
	var ttl time.Duration

	if processing {
		value = "true"
		ttl = 30 * time.Minute
	} else {
		value = "false"
		ttl = 1 * time.Second
	}

	err := r.client.Set(r.ctx, key, value, ttl).Err()
	if err != nil {
		return fmt.Errorf("error setting CRL processing status: %v", err)
	}

	return nil
}

func (r *RedisClient) IsCRLProcessing(url string) (bool, error) {
	key := fmt.Sprintf("crl_processing:%s", url)

	val, err := r.client.Get(r.ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("error getting CRL processing status: %v", err)
	}

	return val == "true", nil
}

func (r *RedisClient) IncrementStats(key string) error {
	err := r.client.Incr(r.ctx, key).Err()
	if err != nil {
		return fmt.Errorf("error incrementing stats: %v", err)
	}
	return nil
}

func (r *RedisClient) GetStats() (map[string]interface{}, error) {
	keys := []string{
		"stats:requests_total",
		"stats:cache_hits",
		"stats:cache_misses",
		"stats:crls_processed",
	}

	pipe := r.client.Pipeline()
	results := make(map[string]*redis.StringCmd)

	for _, key := range keys {
		results[key] = pipe.Get(r.ctx, key)
	}

	_, err := pipe.Exec(r.ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("error getting stats: %v", err)
	}

	stats := make(map[string]interface{})
	for key, cmd := range results {
		val, err := cmd.Result()
		if err == redis.Nil {
			stats[key] = 0
		} else if err != nil {
			stats[key] = 0
		} else {
			stats[key] = val
		}
	}

	return stats, nil
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}
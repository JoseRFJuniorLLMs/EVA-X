package multitenancy

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrNoTenantID         = errors.New("tenant_id not found in context")
	ErrInvalidToken       = errors.New("invalid JWT token")
	ErrMissingTenantClaim = errors.New("tenant_id claim missing in JWT")
)

// TenantContextKey is the key for tenant_id in context
type TenantContextKey struct{}

// Middleware extracts tenant_id from JWT and injects into context
type Middleware struct {
	jwtSecret []byte
}

// NewMiddleware creates a new multi-tenancy middleware
func NewMiddleware(jwtSecret string) *Middleware {
	return &Middleware{
		jwtSecret: []byte(jwtSecret),
	}
}

// ExtractTenantFromJWT extracts tenant_id from JWT token
func (m *Middleware) ExtractTenantFromJWT(tokenString string) (string, error) {
	// Remove "Bearer " prefix if present
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.jwtSecret, nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		tenantID, ok := claims["tenant_id"].(string)
		if !ok {
			return "", ErrMissingTenantClaim
		}
		return tenantID, nil
	}

	return "", ErrInvalidToken
}

// InjectTenantContext injects tenant_id into context
func InjectTenantContext(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantContextKey{}, tenantID)
}

// GetTenantFromContext retrieves tenant_id from context
func GetTenantFromContext(ctx context.Context) (string, error) {
	tenantID, ok := ctx.Value(TenantContextKey{}).(string)
	if !ok {
		return "", ErrNoTenantID
	}
	return tenantID, nil
}

// ValidateIsolation ensures query results belong to the tenant
func ValidateIsolation(ctx context.Context, db *sql.DB, table string, id int64) error {
	tenantID, err := GetTenantFromContext(ctx)
	if err != nil {
		return err
	}

	var count int
	query := "SELECT COUNT(*) FROM " + table + " WHERE id = $1 AND tenant_id = $2"
	err = db.QueryRowContext(ctx, query, id, tenantID).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		log.Printf("⚠️ [SECURITY] Tenant %s attempted to access %s.%d (not owned)", tenantID, table, id)
		return errors.New("resource not found or access denied")
	}

	return nil
}

// WrapQueryWithTenant wraps a SQL query to filter by tenant_id
func WrapQueryWithTenant(ctx context.Context, baseQuery string) (string, []interface{}, error) {
	tenantID, err := GetTenantFromContext(ctx)
	if err != nil {
		return "", nil, err
	}

	// Simple approach: append WHERE tenant_id = $N
	// In production, use proper query builder
	wrappedQuery := baseQuery + " AND tenant_id = $1"

	return wrappedQuery, []interface{}{tenantID}, nil
}

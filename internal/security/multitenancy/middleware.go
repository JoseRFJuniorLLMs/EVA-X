// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package multitenancy

import (
	"context"
	"errors"
	"log"
	"strings"

	"eva/internal/brainstem/database"
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

// allowedTables defines the set of valid table names for tenant isolation queries.
// This prevents SQL injection via table name concatenation.
var allowedTables = map[string]bool{
	"users":          true,
	"idosos":         true,
	"cuidadores":     true,
	"sessions":       true,
	"agendamentos":   true,
	"alertas":        true,
	"crisis_records": true,
	"vital_signs":    true,
	"medications":    true,
}

// ValidateIsolation ensures query results belong to the tenant
func ValidateIsolation(ctx context.Context, db *database.DB, table string, id int64) error {
	if !allowedTables[table] {
		log.Printf("[SECURITY] Rejected invalid table name in isolation check: %s", table)
		return errors.New("invalid resource type")
	}

	tenantID, err := GetTenantFromContext(ctx)
	if err != nil {
		return err
	}

	rows, err := db.QueryByLabel(ctx, table,
		" AND n.id = $id AND n.tenant_id = $tenant_id",
		map[string]interface{}{"id": id, "tenant_id": tenantID}, 1)
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		log.Printf("[SECURITY] Tenant %s attempted to access %s.%d (not owned)", tenantID, table, id)
		return errors.New("resource not found or access denied")
	}

	return nil
}

// WrapQueryWithTenant appends a tenant_id filter to an NQL WHERE clause
// and returns the extra clause plus params map for use with QueryByLabel.
func WrapQueryWithTenant(ctx context.Context, extraWhere string) (string, map[string]interface{}, error) {
	tenantID, err := GetTenantFromContext(ctx)
	if err != nil {
		return "", nil, err
	}

	wrappedWhere := extraWhere + " AND n.tenant_id = $tenant_id"

	return wrappedWhere, map[string]interface{}{"tenant_id": tenantID}, nil
}

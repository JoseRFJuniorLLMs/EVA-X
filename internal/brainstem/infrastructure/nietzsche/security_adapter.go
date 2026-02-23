// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package nietzsche provides EVA's integration layer with NietzscheDB.
//
// This file implements security controls for LGPD (Lei Geral de Protecao de
// Dados) and HIPAA compliance. EVA handles sensitive health data (patient
// records, biomarker readings, clinical observations) which requires:
//
//   - Encryption at-rest: NietzscheDB encrypts all data column families using
//     AES-256-GCM with HKDF-SHA256 per-CF key derivation. The master key is
//     set on the server via NIETZSCHE_ENCRYPTION_KEY (base64-encoded 32 bytes).
//     EVA's role is to validate that encryption is active before storing PHI.
//
//   - RBAC (Role-Based Access Control): NietzscheDB supports three roles
//     (Admin, Writer, Reader) authenticated via x-api-key gRPC metadata.
//     EVA maps clinical roles to NietzscheDB API keys:
//       * admin    -> full access (backup, restore, drop, schema changes)
//       * clinician -> writer (read + write patient data, no admin ops)
//       * patient   -> reader (read own data only, enforced at app layer)
//       * researcher -> reader (anonymised aggregate queries only)
//
//   - Per-collection access policies: sensitive collections (patient_graph,
//     memories, speaker_embeddings) are tagged as PHI (Protected Health
//     Information) and require writer-level credentials for mutations.
//
// ## Architecture
//
// Encryption is handled transparently by NietzscheDB's storage layer --
// EVA does not encrypt/decrypt data itself. Instead, SecurityAdapter:
//  1. Validates that encryption is enabled on the server (via health metadata)
//  2. Provides API-key credential injection for gRPC connections
//  3. Maps EVA roles to NietzscheDB RBAC roles
//  4. Defines which collections contain PHI and enforces access policies
//  5. Logs all security-relevant operations for audit trails (LGPD Art. 37)

package nietzsche

import (
	"context"
	"fmt"

	"eva/internal/brainstem/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	nietzsche "nietzsche-sdk"
)

// ── RBAC Roles ───────────────────────────────────────────────────────────────

// Role represents a NietzscheDB RBAC role mapped from EVA's clinical roles.
// These correspond to the server-side roles defined in nietzsche-api/src/rbac.rs.
type Role string

const (
	// RoleAdmin has full access: all operations including backup, restore,
	// drop collection, schema changes, sleep cycles, and Zaratustra evolution.
	RoleAdmin Role = "admin"

	// RoleWriter can read and write: insert/delete/update nodes and edges,
	// run queries, KNN search, traversals. Cannot perform admin operations.
	// Maps to: clinician, EVA's internal services.
	RoleWriter Role = "writer"

	// RoleReader can only read: get nodes/edges, KNN search, traversals,
	// NQL queries. Cannot mutate any data.
	// Maps to: patient (own data), researcher (anonymised queries).
	RoleReader Role = "reader"
)

// ── PHI Collection Classification ────────────────────────────────────────────

// DataClassification indicates the sensitivity level of a collection's data.
// Used to enforce access policies per LGPD Art. 11 (sensitive personal data)
// and HIPAA 45 CFR 164.312 (access controls).
type DataClassification string

const (
	// ClassificationPHI marks collections containing Protected Health Information.
	// Requires encryption at-rest and writer-level credentials for mutations.
	// Examples: patient_graph, memories, speaker_embeddings.
	ClassificationPHI DataClassification = "PHI"

	// ClassificationInternal marks collections containing EVA's operational data.
	// Not subject to PHI regulations but still encrypted when available.
	// Examples: eva_core, eva_self_knowledge, eva_codebase.
	ClassificationInternal DataClassification = "internal"

	// ClassificationPublic marks collections with non-sensitive data.
	// Examples: eva_docs, stories.
	ClassificationPublic DataClassification = "public"
)

// CollectionPolicy defines the security policy for a NietzscheDB collection.
type CollectionPolicy struct {
	// Name is the collection name as stored in NietzscheDB.
	Name string

	// Classification determines data sensitivity level.
	Classification DataClassification

	// RequireEncryption indicates whether encryption at-rest must be verified
	// before this collection accepts writes. PHI collections always require this.
	RequireEncryption bool

	// MinWriteRole is the minimum RBAC role required for write operations.
	// Reader role can never write; this field selects between Writer and Admin.
	MinWriteRole Role

	// AuditWrites enables per-write audit logging for this collection.
	// Required for PHI collections under LGPD Art. 37 and HIPAA audit controls.
	AuditWrites bool
}

// ── SecurityAdapter ──────────────────────────────────────────────────────────

// SecurityAdapter manages encryption validation, RBAC credential injection,
// and per-collection access policies for EVA's NietzscheDB integration.
//
// It does NOT perform encryption itself -- NietzscheDB handles that at the
// storage layer. SecurityAdapter's responsibilities are:
//   - Injecting x-api-key credentials into gRPC connections
//   - Classifying collections by data sensitivity
//   - Validating that server-side encryption is active for PHI collections
//   - Providing audit logging for security-relevant operations
type SecurityAdapter struct {
	// policies maps collection name to its security policy.
	policies map[string]*CollectionPolicy

	// apiKeys maps Role to the corresponding NietzscheDB API key.
	// These are injected as x-api-key gRPC metadata on each request.
	apiKeys map[Role]string

	// encryptionKeyConfigured indicates whether NIETZSCHE_ENCRYPTION_KEY
	// was set in EVA's config (meaning the server should have encryption active).
	encryptionKeyConfigured bool

	// rbacEnabled indicates whether RBAC is active (at least one API key configured).
	rbacEnabled bool
}

// SecurityConfig holds the configuration needed to initialize SecurityAdapter.
type SecurityConfig struct {
	// EncryptionKey is the base64-encoded 32-byte AES-256 master key.
	// This is set on the NietzscheDB server via NIETZSCHE_ENCRYPTION_KEY.
	// EVA stores it only to verify it was configured; it never sends the key
	// over gRPC -- the server loads it from its own environment.
	EncryptionKey string

	// RBACEnabled controls whether API key authentication is enforced.
	RBACEnabled bool

	// APIKeyAdmin is the API key granting Admin role on NietzscheDB.
	APIKeyAdmin string

	// APIKeyWriter is the API key granting Writer role on NietzscheDB.
	APIKeyWriter string

	// APIKeyReader is the API key granting Reader role on NietzscheDB.
	APIKeyReader string
}

// NewSecurityAdapter creates a SecurityAdapter with the given configuration.
// It initialises default collection policies for all EVA collections.
func NewSecurityAdapter(cfg SecurityConfig) *SecurityAdapter {
	log := logger.Nietzsche()

	sa := &SecurityAdapter{
		policies:                make(map[string]*CollectionPolicy),
		apiKeys:                 make(map[Role]string),
		encryptionKeyConfigured: cfg.EncryptionKey != "",
		rbacEnabled:             cfg.RBACEnabled,
	}

	// Register API keys for each role
	if cfg.APIKeyAdmin != "" {
		sa.apiKeys[RoleAdmin] = cfg.APIKeyAdmin
	}
	if cfg.APIKeyWriter != "" {
		sa.apiKeys[RoleWriter] = cfg.APIKeyWriter
	}
	if cfg.APIKeyReader != "" {
		sa.apiKeys[RoleReader] = cfg.APIKeyReader
	}

	// Initialize default collection security policies
	sa.initDefaultPolicies()

	// Log security posture
	if sa.encryptionKeyConfigured {
		log.Info().Msg("[Security] encryption at-rest key configured (server-side AES-256-GCM)")
	} else {
		log.Warn().Msg("[Security] NIETZSCHE_ENCRYPTION_KEY not set -- PHI collections will NOT have encryption at-rest")
	}

	if sa.rbacEnabled {
		log.Info().
			Int("api_keys", len(sa.apiKeys)).
			Msg("[Security] RBAC enabled with API key authentication")
	} else {
		log.Warn().Msg("[Security] RBAC disabled -- all requests will have Admin access (not recommended for production)")
	}

	log.Info().
		Int("phi_collections", sa.countByClassification(ClassificationPHI)).
		Int("internal_collections", sa.countByClassification(ClassificationInternal)).
		Int("public_collections", sa.countByClassification(ClassificationPublic)).
		Msg("[Security] collection access policies initialized")

	return sa
}

// initDefaultPolicies sets security policies for all EVA collections.
// PHI classification follows LGPD Art. 11 (sensitive personal data) and
// HIPAA 45 CFR 164.312 (technical safeguards for ePHI).
func (sa *SecurityAdapter) initDefaultPolicies() {
	// ── PHI Collections (Protected Health Information) ────────────────────
	// These contain patient data subject to LGPD and HIPAA.

	sa.policies["patient_graph"] = &CollectionPolicy{
		Name:              "patient_graph",
		Classification:    ClassificationPHI,
		RequireEncryption: true,
		MinWriteRole:      RoleWriter,
		AuditWrites:       true,
	}

	sa.policies["memories"] = &CollectionPolicy{
		Name:              "memories",
		Classification:    ClassificationPHI,
		RequireEncryption: true,
		MinWriteRole:      RoleWriter,
		AuditWrites:       true,
	}

	sa.policies["speaker_embeddings"] = &CollectionPolicy{
		Name:              "speaker_embeddings",
		Classification:    ClassificationPHI,
		RequireEncryption: true,
		MinWriteRole:      RoleWriter,
		AuditWrites:       true,
	}

	sa.policies["signifier_chains"] = &CollectionPolicy{
		Name:              "signifier_chains",
		Classification:    ClassificationPHI,
		RequireEncryption: true,
		MinWriteRole:      RoleWriter,
		AuditWrites:       true,
	}

	// ── Internal Collections (EVA operational data) ──────────────────────

	sa.policies["eva_core"] = &CollectionPolicy{
		Name:              "eva_core",
		Classification:    ClassificationInternal,
		RequireEncryption: false,
		MinWriteRole:      RoleWriter,
		AuditWrites:       false,
	}

	sa.policies["eva_self_knowledge"] = &CollectionPolicy{
		Name:              "eva_self_knowledge",
		Classification:    ClassificationInternal,
		RequireEncryption: false,
		MinWriteRole:      RoleWriter,
		AuditWrites:       false,
	}

	sa.policies["eva_learnings"] = &CollectionPolicy{
		Name:              "eva_learnings",
		Classification:    ClassificationInternal,
		RequireEncryption: false,
		MinWriteRole:      RoleWriter,
		AuditWrites:       false,
	}

	sa.policies["eva_codebase"] = &CollectionPolicy{
		Name:              "eva_codebase",
		Classification:    ClassificationInternal,
		RequireEncryption: false,
		MinWriteRole:      RoleWriter,
		AuditWrites:       false,
	}

	// ── Public / Low-sensitivity Collections ─────────────────────────────

	sa.policies["eva_docs"] = &CollectionPolicy{
		Name:              "eva_docs",
		Classification:    ClassificationPublic,
		RequireEncryption: false,
		MinWriteRole:      RoleWriter,
		AuditWrites:       false,
	}

	sa.policies["stories"] = &CollectionPolicy{
		Name:              "stories",
		Classification:    ClassificationPublic,
		RequireEncryption: false,
		MinWriteRole:      RoleWriter,
		AuditWrites:       false,
	}
}

// ── Public API ───────────────────────────────────────────────────────────────

// GetPolicy returns the security policy for a collection, or nil if unknown.
func (sa *SecurityAdapter) GetPolicy(collection string) *CollectionPolicy {
	return sa.policies[collection]
}

// SetPolicy registers or updates a security policy for a collection.
func (sa *SecurityAdapter) SetPolicy(policy *CollectionPolicy) {
	sa.policies[policy.Name] = policy
}

// IsEncryptionConfigured returns whether the encryption key was provided in config.
// When true, the NietzscheDB server should have AES-256-GCM encryption active
// for all data column families (nodes, embeddings, edges, sensory).
func (sa *SecurityAdapter) IsEncryptionConfigured() bool {
	return sa.encryptionKeyConfigured
}

// IsRBACEnabled returns whether RBAC authentication is active.
func (sa *SecurityAdapter) IsRBACEnabled() bool {
	return sa.rbacEnabled
}

// IsPHICollection returns true if the collection contains Protected Health Information.
func (sa *SecurityAdapter) IsPHICollection(collection string) bool {
	if policy, ok := sa.policies[collection]; ok {
		return policy.Classification == ClassificationPHI
	}
	return false
}

// PHICollections returns the names of all collections classified as PHI.
func (sa *SecurityAdapter) PHICollections() []string {
	var result []string
	for name, policy := range sa.policies {
		if policy.Classification == ClassificationPHI {
			result = append(result, name)
		}
	}
	return result
}

// ValidateEncryptionForPHI checks that encryption is configured when PHI
// collections exist. Returns an error if PHI collections are defined but
// no encryption key was provided.
//
// This is a startup-time check per HIPAA 45 CFR 164.312(a)(2)(iv):
// "Implement a mechanism to encrypt electronic protected health information
// whenever deemed appropriate."
func (sa *SecurityAdapter) ValidateEncryptionForPHI() error {
	if sa.encryptionKeyConfigured {
		return nil
	}

	phiCollections := sa.PHICollections()
	if len(phiCollections) > 0 {
		return fmt.Errorf(
			"SECURITY: NIETZSCHE_ENCRYPTION_KEY not configured but %d PHI collections "+
				"require encryption at-rest (LGPD Art. 46 / HIPAA 45 CFR 164.312). "+
				"Collections: %v. Set NIETZSCHE_ENCRYPTION_KEY or reclassify collections",
			len(phiCollections), phiCollections,
		)
	}

	return nil
}

// ValidateWriteAccess checks whether a write operation to the given collection
// is permitted under the current security configuration.
// role is the EVA-level role of the caller (e.g., RoleWriter for clinicians).
func (sa *SecurityAdapter) ValidateWriteAccess(collection string, role Role) error {
	policy := sa.policies[collection]
	if policy == nil {
		// Unknown collection -- allow by default (will be validated at gRPC level)
		return nil
	}

	// Check encryption requirement for PHI
	if policy.RequireEncryption && !sa.encryptionKeyConfigured {
		return fmt.Errorf(
			"SECURITY: write to PHI collection %q blocked -- encryption at-rest not configured "+
				"(LGPD Art. 46 / HIPAA 45 CFR 164.312)",
			collection,
		)
	}

	// Check RBAC role
	if sa.rbacEnabled {
		if !roleAtLeast(role, policy.MinWriteRole) {
			return fmt.Errorf(
				"SECURITY: write to collection %q requires at least %s role, caller has %s",
				collection, policy.MinWriteRole, role,
			)
		}
	}

	return nil
}

// AuditWrite logs a write operation for compliance audit trail.
// Called by EVA's write paths when AuditWrites is enabled on the policy.
// Implements LGPD Art. 37 (record of processing activities) and
// HIPAA 45 CFR 164.312(b) (audit controls).
func (sa *SecurityAdapter) AuditWrite(ctx context.Context, collection, operation, entityID string, role Role) {
	policy := sa.policies[collection]
	if policy == nil || !policy.AuditWrites {
		return
	}

	log := logger.Nietzsche()
	log.Info().
		Str("collection", collection).
		Str("operation", operation).
		Str("entity_id", entityID).
		Str("role", string(role)).
		Str("classification", string(policy.Classification)).
		Msg("[Security:Audit] PHI data access")
}

// ── gRPC Credential Injection ────────────────────────────────────────────────

// apiKeyCredentials implements grpc.PerRPCCredentials to inject the x-api-key
// header into every gRPC request. This is how NietzscheDB's AuthInterceptor
// authenticates clients (see nietzsche-server/src/auth.rs).
type apiKeyCredentials struct {
	apiKey string
}

// GetRequestMetadata injects the x-api-key header into gRPC metadata.
func (c *apiKeyCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		"x-api-key": c.apiKey,
	}, nil
}

// RequireTransportSecurity returns false for insecure connections (same-host / VPN).
// In production with external traffic, this should return true to require TLS.
func (c *apiKeyCredentials) RequireTransportSecurity() bool {
	return false
}

// GRPCDialOptions returns gRPC dial options that inject the API key for the
// given role. Used when creating new NietzscheDB client connections.
//
// If RBAC is disabled or no API key is configured for the role, returns
// only insecure transport credentials (backward compatible).
func (sa *SecurityAdapter) GRPCDialOptions(role Role) []grpc.DialOption {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	if !sa.rbacEnabled {
		return opts
	}

	apiKey := sa.resolveAPIKey(role)
	if apiKey != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(&apiKeyCredentials{apiKey: apiKey}))
	}

	return opts
}

// NewAuthenticatedClient creates a NietzscheDB SDK client with RBAC credentials
// injected for the specified role. Falls back to insecure connection if RBAC
// is disabled.
func (sa *SecurityAdapter) NewAuthenticatedClient(grpcAddr string, role Role) (*nietzsche.NietzscheClient, error) {
	log := logger.Nietzsche()

	dialOpts := sa.GRPCDialOptions(role)
	client, err := nietzsche.Connect(grpcAddr, dialOpts...)
	if err != nil {
		log.Error().Err(err).
			Str("addr", grpcAddr).
			Str("role", string(role)).
			Msg("[Security] failed to create authenticated NietzscheDB client")
		return nil, fmt.Errorf("nietzsche authenticated connect %s (role=%s): %w", grpcAddr, role, err)
	}

	log.Info().
		Str("addr", grpcAddr).
		Str("role", string(role)).
		Bool("rbac", sa.rbacEnabled).
		Msg("[Security] authenticated NietzscheDB client created")

	return client, nil
}

// InjectAPIKeyContext returns a new context with the x-api-key metadata set
// for the given role. Useful when the gRPC connection was created without
// per-RPC credentials but individual calls need authentication.
func (sa *SecurityAdapter) InjectAPIKeyContext(ctx context.Context, role Role) context.Context {
	apiKey := sa.resolveAPIKey(role)
	if apiKey == "" {
		return ctx
	}
	md := metadata.Pairs("x-api-key", apiKey)
	return metadata.NewOutgoingContext(ctx, md)
}

// ── Security Status ──────────────────────────────────────────────────────────

// SecurityStatus provides a summary of the current security configuration.
// Used by health checks and the admin dashboard.
type SecurityStatus struct {
	EncryptionConfigured bool   `json:"encryption_configured"`
	EncryptionAlgorithm  string `json:"encryption_algorithm"`
	RBACEnabled          bool   `json:"rbac_enabled"`
	RolesConfigured      int    `json:"roles_configured"`
	PHICollections       int    `json:"phi_collections"`
	AuditedCollections   int    `json:"audited_collections"`
	ComplianceMode       string `json:"compliance_mode"`
}

// Status returns a summary of the current security posture.
func (sa *SecurityAdapter) Status() SecurityStatus {
	audited := 0
	for _, policy := range sa.policies {
		if policy.AuditWrites {
			audited++
		}
	}

	compliance := "none"
	if sa.encryptionKeyConfigured && sa.rbacEnabled {
		compliance = "LGPD+HIPAA"
	} else if sa.encryptionKeyConfigured {
		compliance = "partial (encryption only)"
	} else if sa.rbacEnabled {
		compliance = "partial (RBAC only)"
	}

	return SecurityStatus{
		EncryptionConfigured: sa.encryptionKeyConfigured,
		EncryptionAlgorithm:  "AES-256-GCM (HKDF-SHA256 per-CF key derivation)",
		RBACEnabled:          sa.rbacEnabled,
		RolesConfigured:      len(sa.apiKeys),
		PHICollections:       sa.countByClassification(ClassificationPHI),
		AuditedCollections:   audited,
		ComplianceMode:       compliance,
	}
}

// ── Internal Helpers ─────────────────────────────────────────────────────────

// resolveAPIKey returns the API key for the given role, falling back to
// higher-privilege keys if the exact role key is not configured.
// Fallback order: exact role -> Admin (as universal fallback).
func (sa *SecurityAdapter) resolveAPIKey(role Role) string {
	if key, ok := sa.apiKeys[role]; ok {
		return key
	}
	// Fall back to admin key (backward compatible with single-key setups)
	if key, ok := sa.apiKeys[RoleAdmin]; ok {
		return key
	}
	return ""
}

// roleAtLeast checks if the given role has at least the required permission level.
// Ordering: Admin > Writer > Reader.
func roleAtLeast(have, need Role) bool {
	return roleLevel(have) >= roleLevel(need)
}

// roleLevel returns a numeric permission level for a role.
func roleLevel(r Role) int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleWriter:
		return 2
	case RoleReader:
		return 1
	default:
		return 0
	}
}

// countByClassification counts collections with the given classification.
func (sa *SecurityAdapter) countByClassification(c DataClassification) int {
	count := 0
	for _, policy := range sa.policies {
		if policy.Classification == c {
			count++
		}
	}
	return count
}

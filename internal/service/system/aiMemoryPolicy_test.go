package system

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	aidomain "personal_assistant/internal/domain/ai"
)

func TestAIMemoryPolicyResolveScopeRejectsOrgWithoutApprovedScope(t *testing.T) {
	policy := aiMemoryPolicy{}
	orgID := uint(23)

	decision := policy.ResolveScope(
		aidomain.MemoryScopeInput{
			ScopeType: aidomain.MemoryScopeOrg,
			OrgID:     &orgID,
		},
		aidomain.MemoryAccessContext{
			Principal: aidomain.AIToolPrincipal{
				UserID:       7,
				CurrentOrgID: &orgID,
			},
		},
	)

	if decision.Allowed {
		t.Fatalf("ResolveScope() allowed = true, want false")
	}
	if decision.ReasonCode != aidomain.MemoryReasonDenyPermissionDependencyMissing {
		t.Fatalf("ResolveScope() reason_code = %q, want %q", decision.ReasonCode, aidomain.MemoryReasonDenyPermissionDependencyMissing)
	}
}

func TestAIMemoryPolicyShouldOverrideFactPrefersExplicitUserPreferenceForSelf(t *testing.T) {
	policy := aiMemoryPolicy{}

	decision := policy.ShouldOverrideFact(
		aidomain.MemoryFactVersion{
			ValueJSON:  `{"style":"detailed"}`,
			SourceKind: aidomain.MemorySourceAdminSet,
		},
		aidomain.MemoryFactVersion{
			ValueJSON:  `{"style":"brief"}`,
			SourceKind: aidomain.MemorySourceExplicitUserStatement,
		},
		aidomain.MemoryScopeSelf,
		aidomain.MemoryNamespaceUserPreference,
	)

	if !decision.Allowed {
		t.Fatalf("ShouldOverrideFact() allowed = false, want true")
	}
	if decision.ReasonCode != aidomain.MemoryReasonOverrideExplicitUserStatement {
		t.Fatalf("ShouldOverrideFact() reason_code = %q, want %q", decision.ReasonCode, aidomain.MemoryReasonOverrideExplicitUserStatement)
	}
}

func TestAIMemoryPolicyShouldOverrideFactPrefersAdminSetForOrgMemory(t *testing.T) {
	policy := aiMemoryPolicy{}

	decision := policy.ShouldOverrideFact(
		aidomain.MemoryFactVersion{
			ValueJSON:  `{"tone":"mentor"}`,
			SourceKind: aidomain.MemorySourceExplicitUserStatement,
		},
		aidomain.MemoryFactVersion{
			ValueJSON:  `{"tone":"strict"}`,
			SourceKind: aidomain.MemorySourceAdminSet,
		},
		aidomain.MemoryScopeOrg,
		aidomain.MemoryNamespaceOrgProfile,
	)

	if !decision.Allowed {
		t.Fatalf("ShouldOverrideFact() allowed = false, want true")
	}
	if decision.ReasonCode != aidomain.MemoryReasonOverrideAdminSet {
		t.Fatalf("ShouldOverrideFact() reason_code = %q, want %q", decision.ReasonCode, aidomain.MemoryReasonOverrideAdminSet)
	}
}

func TestAIMemoryPolicyShouldStoreDocumentRejectsRawTracePayload(t *testing.T) {
	policy := aiMemoryPolicy{}
	userID := uint(9)

	decision := policy.ShouldStoreDocument(
		aidomain.MemoryDocumentCandidate{
			ScopeType:   aidomain.MemoryScopeSelf,
			UserID:      &userID,
			MemoryType:  aidomain.MemoryTypeSemantic,
			Topic:       "trace",
			Summary:     "trace payload",
			ContentText: `{"span":"full"}`,
			SourceKind:  aidomain.MemorySourceRawTracePayload,
		},
		aidomain.MemoryAccessContext{
			Principal: aidomain.AIToolPrincipal{UserID: userID},
		},
	)

	if decision.Allowed {
		t.Fatalf("ShouldStoreDocument() allowed = true, want false")
	}
	if decision.ReasonCode != aidomain.MemoryReasonDenyForbiddenSource {
		t.Fatalf("ShouldStoreDocument() reason_code = %q, want %q", decision.ReasonCode, aidomain.MemoryReasonDenyForbiddenSource)
	}
}

func TestAIMemoryPolicyShouldStoreDocumentBuildsDedupMetadata(t *testing.T) {
	policy := aiMemoryPolicy{}
	orgID := uint(24)

	decision := policy.ShouldStoreDocument(
		aidomain.MemoryDocumentCandidate{
			ScopeType:   aidomain.MemoryScopeOrg,
			OrgID:       &orgID,
			MemoryType:  aidomain.MemoryTypeFAQ,
			Topic:       "deploy",
			Title:       "FAQ",
			Summary:     "  Deploy with migration  ",
			ContentText: "Use docker compose and run migrations before restart.",
			SourceKind:  "faq_import",
			SourceID:    "faq-001",
		},
		aidomain.MemoryAccessContext{
			Principal:            aidomain.AIToolPrincipal{UserID: 8, CurrentOrgID: &orgID},
			ApprovedOrgScopeKeys: []string{aidomain.BuildOrgMemoryScopeKey(orgID)},
		},
	)

	if !decision.Allowed {
		t.Fatalf("ShouldStoreDocument() allowed = false, want true, reason=%s", decision.Reason)
	}
	if decision.ReasonCode != aidomain.MemoryReasonAllowStoreDocument {
		t.Fatalf("ShouldStoreDocument() reason_code = %q, want %q", decision.ReasonCode, aidomain.MemoryReasonAllowStoreDocument)
	}

	wantContentHash := sha256Hex(normalizeWhitespace("Use docker compose and run migrations before restart."))
	wantSummaryHash := sha256Hex(normalizeWhitespace("  Deploy with migration  "))
	wantDedupKey := "src:" + sha256Hex(strings.ToLower(strings.Join([]string{"faq_import", "faq-001", "deploy"}, "\n")))

	if decision.ContentHash != wantContentHash {
		t.Fatalf("content_hash = %q, want %q", decision.ContentHash, wantContentHash)
	}
	if decision.SummaryHash != wantSummaryHash {
		t.Fatalf("summary_hash = %q, want %q", decision.SummaryHash, wantSummaryHash)
	}
	if decision.DedupKey != wantDedupKey {
		t.Fatalf("dedup_key = %q, want %q", decision.DedupKey, wantDedupKey)
	}
}

func TestAIMemoryPolicyCanWriteMemoryFailsClosedWithoutApprovedOrgScope(t *testing.T) {
	policy := aiMemoryPolicy{}
	orgID := uint(25)

	decision := policy.CanWriteMemory(
		aidomain.MemoryAccessTarget{
			ScopeType:  aidomain.MemoryScopeOrg,
			ScopeKey:   aidomain.BuildOrgMemoryScopeKey(orgID),
			Visibility: aidomain.MemoryVisibilityOrg,
			OrgID:      &orgID,
		},
		aidomain.MemoryAccessContext{
			Principal: aidomain.AIToolPrincipal{
				UserID:       7,
				CurrentOrgID: &orgID,
			},
		},
	)

	if decision.Allowed {
		t.Fatalf("CanWriteMemory() allowed = true, want false")
	}
	if decision.ReasonCode != aidomain.MemoryReasonDenyPermissionDependencyMissing {
		t.Fatalf("CanWriteMemory() reason_code = %q, want %q", decision.ReasonCode, aidomain.MemoryReasonDenyPermissionDependencyMissing)
	}
}

func TestAIMemoryPolicyCanReadMemoryRejectsPlatformOpsForNonSuperAdmin(t *testing.T) {
	policy := aiMemoryPolicy{}

	decision := policy.CanReadMemory(
		aidomain.MemoryAccessTarget{
			ScopeType:  aidomain.MemoryScopePlatformOps,
			ScopeKey:   aidomain.BuildPlatformOpsMemoryScopeKey(),
			Visibility: aidomain.MemoryVisibilitySuperAdmin,
		},
		aidomain.MemoryAccessContext{
			Principal:        aidomain.AIToolPrincipal{UserID: 11},
			AllowPlatformOps: true,
		},
	)

	if decision.Allowed {
		t.Fatalf("CanReadMemory() allowed = true, want false")
	}
	if decision.ReasonCode != aidomain.MemoryReasonDenyNotSuperAdmin {
		t.Fatalf("CanReadMemory() reason_code = %q, want %q", decision.ReasonCode, aidomain.MemoryReasonDenyNotSuperAdmin)
	}
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

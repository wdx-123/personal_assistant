package system

import (
	"fmt"
	"strings"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
)

// aiMemoryPolicy 收口 writeback 之前的准入、权限和覆盖规则。
type aiMemoryPolicy struct{}

func (p aiMemoryPolicy) ShouldStoreFact(
	candidate aidomain.MemoryFactCandidate,
	access aidomain.MemoryAccessContext,
) aidomain.MemoryDecision {
	if isForbiddenMemorySource(candidate.SourceKind) {
		return denyDecision(
			aidomain.MemoryReasonDenyForbiddenSource,
			fmt.Sprintf("source_kind=%s is forbidden for fact memory", candidate.SourceKind),
		)
	}
	if candidate.TruthConflict {
		return denyDecision(
			aidomain.MemoryReasonDenyTruthConflict,
			"candidate fact conflicts with runtime truth source",
		)
	}
	if candidate.LowValue ||
		strings.TrimSpace(candidate.Namespace) == "" ||
		strings.TrimSpace(candidate.FactKey) == "" ||
		strings.TrimSpace(candidate.FactValueJSON) == "" {
		return denyDecision(
			aidomain.MemoryReasonDenyLowValueContent,
			"fact candidate is empty or marked as low value",
		)
	}

	scopeDecision := p.ResolveScope(aidomain.MemoryScopeInput{
		ScopeType: candidate.ScopeType,
		UserID:    candidate.UserID,
		OrgID:     candidate.OrgID,
	}, access)
	if !scopeDecision.Allowed {
		return scopeDecision.MemoryDecision
	}
	visibilityDecision := p.ResolveVisibility(scopeDecision, candidate.SourceKind)
	if !visibilityDecision.Allowed {
		return visibilityDecision.MemoryDecision
	}
	writeDecision := p.CanWriteMemory(newMemoryAccessTarget(scopeDecision, visibilityDecision), access)
	if !writeDecision.Allowed {
		return writeDecision
	}

	return allowDecision(
		aidomain.MemoryReasonAllowStoreFact,
		fmt.Sprintf("fact candidate is allowed under %s", aidomain.DescribeMemoryScope(scopeDecision.ScopeType, scopeDecision.ScopeKey)),
	)
}

func (p aiMemoryPolicy) ShouldStoreDocument(
	candidate aidomain.MemoryDocumentCandidate,
	access aidomain.MemoryAccessContext,
) aidomain.MemoryDocumentDecision {
	if candidate.MemoryType == aidomain.MemoryTypeSessionSummary {
		return denyDocumentDecision(
			aidomain.MemoryReasonDenyForbiddenSource,
			"session_summary must use conversation_summary storage instead of document memory",
		)
	}
	if isForbiddenMemorySource(candidate.SourceKind) {
		return denyDocumentDecision(
			aidomain.MemoryReasonDenyForbiddenSource,
			fmt.Sprintf("source_kind=%s is forbidden for document memory", candidate.SourceKind),
		)
	}
	if candidate.TruthConflict {
		return denyDocumentDecision(
			aidomain.MemoryReasonDenyTruthConflict,
			"candidate document conflicts with runtime truth source",
		)
	}
	if candidate.LowValue ||
		(strings.TrimSpace(candidate.Summary) == "" && strings.TrimSpace(candidate.ContentText) == "") {
		return denyDocumentDecision(
			aidomain.MemoryReasonDenyLowValueContent,
			"document candidate is empty or marked as low value",
		)
	}

	scopeDecision := p.ResolveScope(aidomain.MemoryScopeInput{
		ScopeType: candidate.ScopeType,
		UserID:    candidate.UserID,
		OrgID:     candidate.OrgID,
	}, access)
	if !scopeDecision.Allowed {
		return denyDocumentDecision(scopeDecision.ReasonCode, scopeDecision.Reason)
	}
	visibilityDecision := p.ResolveVisibility(scopeDecision, candidate.SourceKind)
	if !visibilityDecision.Allowed {
		return denyDocumentDecision(visibilityDecision.ReasonCode, visibilityDecision.Reason)
	}
	writeDecision := p.CanWriteMemory(newMemoryAccessTarget(scopeDecision, visibilityDecision), access)
	if !writeDecision.Allowed {
		return denyDocumentDecision(writeDecision.ReasonCode, writeDecision.Reason)
	}

	contentHash := aidomain.BuildMemoryDocumentContentHash(candidate.ContentText)
	summaryHash := aidomain.BuildMemoryDocumentSummaryHash(candidate.Summary)
	dedupKey := aidomain.BuildMemoryDocumentDedupKey(
		candidate.SourceKind,
		candidate.SourceID,
		candidate.Topic,
		candidate.Summary,
		candidate.ContentText,
	)
	if dedupKey == "" {
		return denyDocumentDecision(
			aidomain.MemoryReasonDenyDuplicateDocument,
			"document candidate does not have a stable dedup key",
		)
	}

	return aidomain.MemoryDocumentDecision{
		MemoryDecision: allowDecision(
			aidomain.MemoryReasonAllowStoreDocument,
			fmt.Sprintf("document candidate is allowed under %s", aidomain.DescribeMemoryScope(scopeDecision.ScopeType, scopeDecision.ScopeKey)),
		),
		ContentHash: contentHash,
		SummaryHash: summaryHash,
		DedupKey:    dedupKey,
	}
}

func (p aiMemoryPolicy) ResolveScope(
	input aidomain.MemoryScopeInput,
	access aidomain.MemoryAccessContext,
) aidomain.MemoryScopeDecision {
	switch input.ScopeType {
	case aidomain.MemoryScopeSelf:
		if access.Principal.UserID == 0 {
			return denyScopeDecision(
				aidomain.MemoryReasonDenyPermissionDependencyMissing,
				"principal.user_id is required for self scope",
			)
		}
		resolvedUserID := access.Principal.UserID
		if input.UserID != nil && *input.UserID > 0 {
			if *input.UserID != access.Principal.UserID {
				return denyScopeDecision(
					aidomain.MemoryReasonDenyScopeMismatch,
					fmt.Sprintf("self scope user_id=%d does not match principal user_id=%d", *input.UserID, access.Principal.UserID),
				)
			}
			resolvedUserID = *input.UserID
		}
		scopeKey := aidomain.BuildSelfMemoryScopeKey(resolvedUserID)
		return aidomain.MemoryScopeDecision{
			MemoryDecision: allowDecision(
				aidomain.MemoryReasonAllowSelfScope,
				fmt.Sprintf("resolved self scope for user_id=%d", resolvedUserID),
			),
			ScopeType: aidomain.MemoryScopeSelf,
			ScopeKey:  scopeKey,
			UserID:    &resolvedUserID,
		}

	case aidomain.MemoryScopeOrg:
		if input.OrgID == nil || *input.OrgID == 0 {
			return denyScopeDecision(
				aidomain.MemoryReasonDenyPermissionDependencyMissing,
				"explicit authorized org_id is required for org scope",
			)
		}
		if len(access.ApprovedOrgScopeKeys) == 0 && len(access.ApprovedOrgIDs) == 0 {
			return denyScopeDecision(
				aidomain.MemoryReasonDenyPermissionDependencyMissing,
				"approved org scopes are required for org memory",
			)
		}
		if !isApprovedOrgScope(*input.OrgID, access) {
			return denyScopeDecision(
				aidomain.MemoryReasonDenyNotInApprovedOrgScope,
				fmt.Sprintf("org_id=%d is not in approved org scope set", *input.OrgID),
			)
		}
		scopeKey := aidomain.BuildOrgMemoryScopeKey(*input.OrgID)
		return aidomain.MemoryScopeDecision{
			MemoryDecision: allowDecision(
				aidomain.MemoryReasonAllowOrgScope,
				fmt.Sprintf("resolved org scope for org_id=%d", *input.OrgID),
			),
			ScopeType: aidomain.MemoryScopeOrg,
			ScopeKey:  scopeKey,
			UserID:    cloneMemoryUintPtr(input.UserID),
			OrgID:     cloneMemoryUintPtr(input.OrgID),
		}

	case aidomain.MemoryScopePlatformOps:
		if !access.AllowPlatformOps {
			return denyScopeDecision(
				aidomain.MemoryReasonDenyPermissionDependencyMissing,
				"platform_ops scope requires explicit authorization result",
			)
		}
		if !access.Principal.IsSuperAdmin {
			return denyScopeDecision(
				aidomain.MemoryReasonDenyNotSuperAdmin,
				"platform_ops scope requires super admin principal",
			)
		}
		scopeKey := aidomain.BuildPlatformOpsMemoryScopeKey()
		return aidomain.MemoryScopeDecision{
			MemoryDecision: allowDecision(
				aidomain.MemoryReasonAllowPlatformOpsScope,
				"resolved platform_ops scope for super admin",
			),
			ScopeType: aidomain.MemoryScopePlatformOps,
			ScopeKey:  scopeKey,
			UserID:    cloneMemoryUintPtr(input.UserID),
		}
	}

	return denyScopeDecision(
		aidomain.MemoryReasonDenyScopeMismatch,
		fmt.Sprintf("unsupported memory scope type=%s", input.ScopeType),
	)
}

func (p aiMemoryPolicy) ResolveVisibility(
	scope aidomain.MemoryScopeDecision,
	_ aidomain.MemorySourceKind,
) aidomain.MemoryVisibilityDecision {
	if !scope.Allowed {
		return aidomain.MemoryVisibilityDecision{MemoryDecision: scope.MemoryDecision}
	}
	switch scope.ScopeType {
	case aidomain.MemoryScopeSelf:
		return aidomain.MemoryVisibilityDecision{
			MemoryDecision: allowDecision(
				aidomain.MemoryReasonAllowSelfScope,
				"self scope maps to self visibility",
			),
			Visibility: aidomain.MemoryVisibilitySelf,
		}
	case aidomain.MemoryScopeOrg:
		return aidomain.MemoryVisibilityDecision{
			MemoryDecision: allowDecision(
				aidomain.MemoryReasonAllowOrgScope,
				"org scope maps to org visibility",
			),
			Visibility: aidomain.MemoryVisibilityOrg,
		}
	case aidomain.MemoryScopePlatformOps:
		return aidomain.MemoryVisibilityDecision{
			MemoryDecision: allowDecision(
				aidomain.MemoryReasonAllowPlatformOpsScope,
				"platform_ops scope maps to super_admin visibility",
			),
			Visibility: aidomain.MemoryVisibilitySuperAdmin,
		}
	}
	return aidomain.MemoryVisibilityDecision{
		MemoryDecision: denyDecision(
			aidomain.MemoryReasonDenyVisibilityMismatch,
			fmt.Sprintf("unsupported visibility mapping for scope_type=%s", scope.ScopeType),
		),
	}
}

func (p aiMemoryPolicy) ResolveTTL(namespace string, memoryType aidomain.MemoryType) aidomain.MemoryTTLDecision {
	now := time.Now()
	switch strings.TrimSpace(namespace) {
	case aidomain.MemoryNamespaceUserPreference, aidomain.MemoryNamespaceOrgProfile,
		aidomain.MemoryNamespaceOpsIncident, aidomain.MemoryNamespaceOpsRunbook:
		return aidomain.MemoryTTLDecision{
			MemoryDecision: allowDecision(
				aidomain.MemoryReasonAllowTTLPersistent,
				fmt.Sprintf("namespace=%s is persistent by policy", namespace),
			),
		}
	case aidomain.MemoryNamespaceOJGoal:
		expiresAt := now.Add(30 * 24 * time.Hour)
		return aidomain.MemoryTTLDecision{
			MemoryDecision: allowDecision(
				aidomain.MemoryReasonAllowTTLExpiring,
				fmt.Sprintf("namespace=%s expires in 30 days", namespace),
			),
			ExpiresAt: &expiresAt,
		}
	case aidomain.MemoryNamespaceOJProfile:
		expiresAt := now.Add(60 * 24 * time.Hour)
		return aidomain.MemoryTTLDecision{
			MemoryDecision: allowDecision(
				aidomain.MemoryReasonAllowTTLExpiring,
				fmt.Sprintf("namespace=%s expires in 60 days", namespace),
			),
			ExpiresAt: &expiresAt,
		}
	case aidomain.MemoryNamespaceOrgLearning:
		expiresAt := now.Add(14 * 24 * time.Hour)
		return aidomain.MemoryTTLDecision{
			MemoryDecision: allowDecision(
				aidomain.MemoryReasonAllowTTLExpiring,
				fmt.Sprintf("namespace=%s expires in 14 days", namespace),
			),
			ExpiresAt: &expiresAt,
		}
	}

	if memoryType == aidomain.MemoryTypeSessionSummary {
		return aidomain.MemoryTTLDecision{
			MemoryDecision: allowDecision(
				aidomain.MemoryReasonAllowTTLPersistent,
				"conversation summary does not rely on ttl",
			),
		}
	}

	return aidomain.MemoryTTLDecision{
		MemoryDecision: allowDecision(
			aidomain.MemoryReasonAllowTTLPersistent,
			"default memory ttl is persistent until explicitly invalidated",
		),
	}
}

func (p aiMemoryPolicy) ShouldOverrideFact(
	current aidomain.MemoryFactVersion,
	candidate aidomain.MemoryFactVersion,
	scopeType aidomain.MemoryScopeType,
	namespace string,
) aidomain.MemoryDecision {
	currentValue := strings.TrimSpace(current.ValueJSON)
	candidateValue := strings.TrimSpace(candidate.ValueJSON)
	if candidateValue == "" {
		return denyDecision(
			aidomain.MemoryReasonDenyLowValueContent,
			"candidate fact value is empty",
		)
	}
	if currentValue == candidateValue {
		return denyDecision(
			aidomain.MemoryReasonSkipSameValue,
			"candidate fact has the same value as current fact",
		)
	}

	currentPriority := memorySourcePriority(scopeType, namespace, current.SourceKind)
	candidatePriority := memorySourcePriority(scopeType, namespace, candidate.SourceKind)
	if candidatePriority < currentPriority {
		return denyDecision(
			aidomain.MemoryReasonSkipLowerPrioritySource,
			fmt.Sprintf("candidate source=%s has lower priority than current source=%s", candidate.SourceKind, current.SourceKind),
		)
	}
	if candidatePriority > currentPriority {
		return allowDecision(overrideReasonCode(candidate.SourceKind), overrideReasonText(candidate.SourceKind))
	}

	return allowDecision(
		aidomain.MemoryReasonAllowSamePriority,
		fmt.Sprintf("candidate source=%s has the same priority and a new value", candidate.SourceKind),
	)
}

func (p aiMemoryPolicy) CanReadMemory(
	target aidomain.MemoryAccessTarget,
	access aidomain.MemoryAccessContext,
) aidomain.MemoryDecision {
	return evaluateMemoryAccess(target, access)
}

func (p aiMemoryPolicy) CanWriteMemory(
	target aidomain.MemoryAccessTarget,
	access aidomain.MemoryAccessContext,
) aidomain.MemoryDecision {
	return evaluateMemoryAccess(target, access)
}

func evaluateMemoryAccess(
	target aidomain.MemoryAccessTarget,
	access aidomain.MemoryAccessContext,
) aidomain.MemoryDecision {
	switch target.ScopeType {
	case aidomain.MemoryScopeSelf:
		if access.Principal.UserID == 0 || target.UserID == nil || *target.UserID == 0 {
			return denyDecision(
				aidomain.MemoryReasonDenyPermissionDependencyMissing,
				"self memory requires principal.user_id and target.user_id",
			)
		}
		expectedScopeKey := aidomain.BuildSelfMemoryScopeKey(*target.UserID)
		if strings.TrimSpace(target.ScopeKey) != expectedScopeKey || *target.UserID != access.Principal.UserID {
			return denyDecision(
				aidomain.MemoryReasonDenyScopeMismatch,
				fmt.Sprintf("self memory target does not match principal user_id=%d", access.Principal.UserID),
			)
		}
		if target.Visibility != aidomain.MemoryVisibilitySelf {
			return denyDecision(
				aidomain.MemoryReasonDenyVisibilityMismatch,
				fmt.Sprintf("self memory requires visibility=%s", aidomain.MemoryVisibilitySelf),
			)
		}
		return allowDecision(
			aidomain.MemoryReasonAllowSelfScope,
			fmt.Sprintf("self memory access allowed for user_id=%d", access.Principal.UserID),
		)

	case aidomain.MemoryScopeOrg:
		if target.OrgID == nil || *target.OrgID == 0 || strings.TrimSpace(target.ScopeKey) == "" {
			return denyDecision(
				aidomain.MemoryReasonDenyPermissionDependencyMissing,
				"org memory requires explicit target org_id and scope_key",
			)
		}
		if len(access.ApprovedOrgScopeKeys) == 0 && len(access.ApprovedOrgIDs) == 0 {
			return denyDecision(
				aidomain.MemoryReasonDenyPermissionDependencyMissing,
				"org memory requires approved org scope input",
			)
		}
		expectedScopeKey := aidomain.BuildOrgMemoryScopeKey(*target.OrgID)
		if target.ScopeKey != expectedScopeKey {
			return denyDecision(
				aidomain.MemoryReasonDenyScopeMismatch,
				fmt.Sprintf("org memory scope_key=%s does not match org_id=%d", target.ScopeKey, *target.OrgID),
			)
		}
		if target.Visibility != aidomain.MemoryVisibilityOrg {
			return denyDecision(
				aidomain.MemoryReasonDenyVisibilityMismatch,
				fmt.Sprintf("org memory requires visibility=%s", aidomain.MemoryVisibilityOrg),
			)
		}
		if !isApprovedOrgScope(*target.OrgID, access) {
			return denyDecision(
				aidomain.MemoryReasonDenyNotInApprovedOrgScope,
				fmt.Sprintf("org_id=%d is not in approved org scope set", *target.OrgID),
			)
		}
		return allowDecision(
			aidomain.MemoryReasonAllowOrgScope,
			fmt.Sprintf("org memory access allowed for org_id=%d", *target.OrgID),
		)

	case aidomain.MemoryScopePlatformOps:
		if !access.AllowPlatformOps {
			return denyDecision(
				aidomain.MemoryReasonDenyPermissionDependencyMissing,
				"platform_ops memory requires explicit authorization input",
			)
		}
		if !access.Principal.IsSuperAdmin {
			return denyDecision(
				aidomain.MemoryReasonDenyNotSuperAdmin,
				"platform_ops memory requires super admin principal",
			)
		}
		if target.ScopeKey != aidomain.BuildPlatformOpsMemoryScopeKey() {
			return denyDecision(
				aidomain.MemoryReasonDenyScopeMismatch,
				fmt.Sprintf("platform_ops memory requires scope_key=%s", aidomain.BuildPlatformOpsMemoryScopeKey()),
			)
		}
		if target.Visibility != aidomain.MemoryVisibilitySuperAdmin {
			return denyDecision(
				aidomain.MemoryReasonDenyVisibilityMismatch,
				fmt.Sprintf("platform_ops memory requires visibility=%s", aidomain.MemoryVisibilitySuperAdmin),
			)
		}
		return allowDecision(
			aidomain.MemoryReasonAllowPlatformOpsScope,
			"platform_ops memory access allowed for super admin",
		)
	}

	return denyDecision(
		aidomain.MemoryReasonDenyScopeMismatch,
		fmt.Sprintf("unsupported memory scope_type=%s", target.ScopeType),
	)
}

func newMemoryAccessTarget(
	scope aidomain.MemoryScopeDecision,
	visibility aidomain.MemoryVisibilityDecision,
) aidomain.MemoryAccessTarget {
	return aidomain.MemoryAccessTarget{
		ScopeType:  scope.ScopeType,
		ScopeKey:   scope.ScopeKey,
		Visibility: visibility.Visibility,
		UserID:     cloneMemoryUintPtr(scope.UserID),
		OrgID:      cloneMemoryUintPtr(scope.OrgID),
	}
}

func isForbiddenMemorySource(sourceKind aidomain.MemorySourceKind) bool {
	switch sourceKind {
	case aidomain.MemorySourceRawTracePayload, aidomain.MemorySourceFullToolOutput:
		return true
	default:
		return false
	}
}

func isApprovedOrgScope(orgID uint, access aidomain.MemoryAccessContext) bool {
	for _, approvedID := range access.ApprovedOrgIDs {
		if approvedID == orgID {
			return true
		}
	}
	scopeKey := aidomain.BuildOrgMemoryScopeKey(orgID)
	for _, approvedScopeKey := range access.ApprovedOrgScopeKeys {
		if strings.TrimSpace(approvedScopeKey) == scopeKey {
			return true
		}
	}
	return false
}

func memorySourcePriority(
	scopeType aidomain.MemoryScopeType,
	namespace string,
	sourceKind aidomain.MemorySourceKind,
) int {
	if scopeType == aidomain.MemoryScopeOrg || scopeType == aidomain.MemoryScopePlatformOps {
		return publicMemorySourcePriority(sourceKind)
	}
	if scopeType == aidomain.MemoryScopeSelf {
		if strings.TrimSpace(namespace) == aidomain.MemoryNamespaceUserPreference {
			return privateMemorySourcePriority(sourceKind)
		}
		return privateMemorySourcePriority(sourceKind)
	}
	return privateMemorySourcePriority(sourceKind)
}

func privateMemorySourcePriority(sourceKind aidomain.MemorySourceKind) int {
	switch sourceKind {
	case aidomain.MemorySourceExplicitUserStatement:
		return 4
	case aidomain.MemorySourceAdminSet:
		return 3
	case aidomain.MemorySourceToolVerifiedSummary:
		return 2
	case aidomain.MemorySourceModelInferred:
		return 1
	default:
		return 0
	}
}

func publicMemorySourcePriority(sourceKind aidomain.MemorySourceKind) int {
	switch sourceKind {
	case aidomain.MemorySourceAdminSet:
		return 4
	case aidomain.MemorySourceExplicitUserStatement:
		return 3
	case aidomain.MemorySourceToolVerifiedSummary:
		return 2
	case aidomain.MemorySourceModelInferred:
		return 1
	default:
		return 0
	}
}

func overrideReasonCode(sourceKind aidomain.MemorySourceKind) string {
	switch sourceKind {
	case aidomain.MemorySourceExplicitUserStatement:
		return aidomain.MemoryReasonOverrideExplicitUserStatement
	case aidomain.MemorySourceAdminSet:
		return aidomain.MemoryReasonOverrideAdminSet
	default:
		return aidomain.MemoryReasonAllowSamePriority
	}
}

func overrideReasonText(sourceKind aidomain.MemorySourceKind) string {
	switch sourceKind {
	case aidomain.MemorySourceExplicitUserStatement:
		return "candidate explicit user statement overrides current fact"
	case aidomain.MemorySourceAdminSet:
		return "candidate admin setting overrides current fact"
	default:
		return fmt.Sprintf("candidate source=%s overrides current fact", sourceKind)
	}
}

func cloneMemoryUintPtr(value *uint) *uint {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

func allowDecision(code string, reason string) aidomain.MemoryDecision {
	return aidomain.MemoryDecision{
		Allowed:    true,
		ReasonCode: code,
		Reason:     reason,
	}
}

func denyDecision(code string, reason string) aidomain.MemoryDecision {
	return aidomain.MemoryDecision{
		Allowed:    false,
		ReasonCode: code,
		Reason:     reason,
	}
}

func denyScopeDecision(code string, reason string) aidomain.MemoryScopeDecision {
	return aidomain.MemoryScopeDecision{
		MemoryDecision: denyDecision(code, reason),
	}
}

func denyDocumentDecision(code string, reason string) aidomain.MemoryDocumentDecision {
	return aidomain.MemoryDocumentDecision{
		MemoryDecision: denyDecision(code, reason),
	}
}

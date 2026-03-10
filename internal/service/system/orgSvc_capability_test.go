package system

import (
	"testing"

	"personal_assistant/internal/model/consts"
)

func TestCapabilityForOrgMemberAction(t *testing.T) {
	cases := map[string]string{
		consts.OrgMemberActionKick:       consts.CapabilityCodeOrgMemberKick,
		consts.OrgMemberActionRecover:    consts.CapabilityCodeOrgMemberRecover,
		consts.OrgMemberActionFreeze:     consts.CapabilityCodeOrgMemberFreeze,
		consts.OrgMemberActionDelete:     consts.CapabilityCodeOrgMemberDelete,
		consts.OrgMemberActionInvite:     consts.CapabilityCodeOrgMemberInvite,
		consts.OrgMemberActionAssignRole: consts.CapabilityCodeOrgMemberAssignRole,
	}

	for action, expected := range cases {
		actual, err := capabilityForOrgMemberAction(action)
		if err != nil {
			t.Fatalf("action %s returned error: %v", action, err)
		}
		if actual != expected {
			t.Fatalf("action %s mapped to %s, want %s", action, actual, expected)
		}
	}

	if _, err := capabilityForOrgMemberAction("unknown"); err == nil {
		t.Fatal("expected unknown action to return error")
	}
}

func TestCapabilityForOrgAction(t *testing.T) {
	cases := map[string]string{
		consts.OrgActionUpdate: consts.CapabilityCodeOrgManageUpdate,
		consts.OrgActionDelete: consts.CapabilityCodeOrgManageDelete,
	}

	for action, expected := range cases {
		actual, err := capabilityForOrgAction(action)
		if err != nil {
			t.Fatalf("action %s returned error: %v", action, err)
		}
		if actual != expected {
			t.Fatalf("action %s mapped to %s, want %s", action, actual, expected)
		}
	}

	if _, err := capabilityForOrgAction("unknown"); err == nil {
		t.Fatal("expected unknown action to return error")
	}
}

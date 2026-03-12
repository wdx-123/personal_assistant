package casbin

import (
	"fmt"
	"strings"

	casbinlib "github.com/casbin/casbin/v2"

	"personal_assistant/global"
)

// ActionAccess 表示访问权限（如访问某个API）
// ActionRead 表示读取权限（如读取某个资源）
// ActionOperate 表示操作权限（如修改/删除某个资源）
const (
	ActionAccess  = "access"
	ActionRead    = "read"
	ActionOperate = "operate"
)

// Permission 表示一个权限项，包含权限主体、对象和动作。
type Permission struct {
	Subject string	// 权限主体（如用户ID@组织ID）
	Object  string	// 权限对象（如资源ID）
	Action  string	// 权限动作（如访问、读取、操作）
}

type PolicySnapshot struct {
	SubjectRoles map[string][]string
	Permissions  []Permission
}

type Service struct {
	enforcer *casbinlib.Enforcer
}

func NewService() *Service {
	return &Service{enforcer: global.CasbinEnforcer}
}

func NewServiceWithEnforcer(enforcer *casbinlib.Enforcer) *Service {
	return &Service{enforcer: enforcer}
}

func BuildSubject(userID, orgID uint) string {
	return fmt.Sprintf("%d@%d", userID, orgID)
}

// ReloadPolicy 从持久化存储重新加载权限数据到内存，适用于权限投影链路中的增量更新场景。
func (s *Service) ReloadPolicy() error {
	enforcer, err := s.requireEnforcer()
	if err != nil {
		return err
	}
	return enforcer.LoadPolicy()
}

// HasPermission 检查权限主体是否具有某个权限（如访问某个API），返回true表示有权限，false表示没有权限或发生错误。
func (s *Service) HasPermission(subject, object, action string) (bool, error) {
	enforcer, err := s.requireEnforcer()
	if err != nil {
		return false, err
	}
	subject = strings.TrimSpace(subject)
	object = strings.TrimSpace(object)
	action = strings.TrimSpace(action)
	if subject == "" || object == "" || action == "" {
		return false, fmt.Errorf("subject, object or action is empty")
	}
	return enforcer.Enforce(subject, object, action)
}

// ReplaceSubjectRoles 替换权限主体的角色绑定（先撤销原有绑定，再绑定新角色）
func (s *Service) ReplaceSubjectRoles(subject string, roles []string) error {
	enforcer, err := s.requireEnforcer()
	if err != nil {
		return err
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return fmt.Errorf("subject is empty")
	}
	if _, err := enforcer.DeleteRolesForUser(subject); err != nil {
		return err
	}
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role == "" {
			continue
		}
		if _, err := enforcer.AddRoleForUser(subject, role); err != nil {
			return err
		}
	}
	return nil
}

// ClearSubjectRoles 从权限主体撤销所有角色绑定（不删除权限主体的直接权限）
func (s *Service) ClearSubjectRoles(subject string) error {
	enforcer, err := s.requireEnforcer()
	if err != nil {
		return err
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return fmt.Errorf("subject is empty")
	}
	_, err = enforcer.DeleteRolesForUser(subject)
	return err
}

// GrantPermissions 给权限主体授予权限
func (s *Service) GrantPermissions(subject string, permissions []Permission) error {
	enforcer, err := s.requireEnforcer()
	if err != nil {
		return err
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return fmt.Errorf("subject is empty")
	}
	for _, permission := range permissions {
		object := strings.TrimSpace(permission.Object)
		action := strings.TrimSpace(permission.Action)
		if object == "" || action == "" {
			continue
		}
		if _, err := enforcer.AddPermissionForUser(subject, object, action); err != nil {
			return err
		}
	}
	return nil
}

// RevokePermissions 从权限主体撤销权限（仅删除指定权限，不影响其他权限和角色绑定）
func (s *Service) ClearAllPolicies() error {
	enforcer, err := s.requireEnforcer()
	if err != nil {
		return err
	}
	enforcer.ClearPolicy()
	if enforcer.GetAdapter() == nil {
		return nil
	}
	return enforcer.SavePolicy()
}

// RebuildFromSnapshot 从权限快照重建权限数据，适用于权限投影链路中的全量重建场景。
func (s *Service) RebuildFromSnapshot(snapshot *PolicySnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("policy snapshot is nil")
	}
	if err := s.ClearAllPolicies(); err != nil {
		return err
	}
	enforcer, err := s.requireEnforcer()
	if err != nil {
		return err
	}

	// 根据权限快照中的主体角色映射和权限列表，逐条添加角色绑定和权限规则到 Casbin 中
	for subject, roles := range snapshot.SubjectRoles {
		subject = strings.TrimSpace(subject)
		if subject == "" {
			continue
		}
		for _, role := range roles {
			role = strings.TrimSpace(role)
			if role == "" {
				continue
			}
			if _, err := enforcer.AddRoleForUser(subject, role); err != nil {
				return err
			}
		}
	}

	// 添加权限规则
	for _, permission := range snapshot.Permissions {
		subject := strings.TrimSpace(permission.Subject)
		object := strings.TrimSpace(permission.Object)
		action := strings.TrimSpace(permission.Action)
		if subject == "" || object == "" || action == "" {
			continue
		}
		if _, err := enforcer.AddPermissionForUser(subject, object, action); err != nil {
			return err
		}
	}
	return nil
}

// requireEnforcer 获取Casbin Enforcer实例，如果未正确初始化则返回错误
func (s *Service) requireEnforcer() (*casbinlib.Enforcer, error) {
	if s == nil || s.enforcer == nil {
		return nil, fmt.Errorf("casbin enforcer is nil")
	}
	return s.enforcer, nil
}

package response

// OrgSimpleItem 组织简要信息
type OrgSimpleItem struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// UserDetailItem 用户详情信息
type UserDetailItem struct {
	ID           uint           `json:"id"`
	UUID         string         `json:"uuid"`
	Username     string         `json:"username"`
	Phone        string         `json:"phone"`
	Email        string         `json:"email"`
	Avatar       string         `json:"avatar"`
	AvatarID     *uint          `json:"avatar_id"`
	Address      string         `json:"address"`
	Signature    string         `json:"signature"`
	Register     int            `json:"register"`
	Freeze       bool           `json:"freeze"`
	Status       int            `json:"status"`
	IsSuperAdmin bool           `json:"is_super_admin"`
	DisabledAt   string         `json:"disabled_at,omitempty"`
	CreatedAt    string         `json:"created_at"`
	UpdatedAt    string         `json:"updated_at"`
	CurrentOrgID *uint          `json:"current_org_id"`
	CurrentOrg   *OrgSimpleItem `json:"current_org"`
}

// UserListItem 用户列表项，用于用户列表分页查询的响应
type UserListItem struct {
	// 用户ID
	ID uint `json:"id"`
	// 用户名
	Username string `json:"username"`
	// 手机号
	Phone string `json:"phone"`
	// 当前组织ID
	CurrentOrgID *uint `json:"current_org_id"`
	// 当前所属组织（嵌套 id + name）
	CurrentOrg *OrgSimpleItem `json:"current_org"`
	// 用户拥有的角色列表（嵌套 id + name）
	Roles []struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	} `json:"roles"`
}

// PageDataUser 用户列表分页响应
type PageDataUser struct {
	List     []*UserListItem `json:"list"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// UserRoleMatrixRoleItem 用户角色矩阵中的单个角色项
type UserRoleMatrixRoleItem struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Code           string `json:"code"`
	IsBuiltin      bool   `json:"is_builtin"`
	MatrixLevel    string `json:"matrix_level"`
	Assignable     bool   `json:"assignable"`
	DisabledReason string `json:"disabled_reason,omitempty"`
}

// UserRoleMatrixItem 用户角色分配矩阵响应
type UserRoleMatrixItem struct {
	AssignedRoleIDs     []uint                   `json:"assigned_role_ids"`
	OperatorMatrixLevel string                   `json:"operator_matrix_level"`
	Roles               []UserRoleMatrixRoleItem `json:"roles"`
}

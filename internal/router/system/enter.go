package system

type RouterGroup struct {
	RefreshTokenRouter
	BaseRouter
	UserRouter
	OrgRouter
	OJRouter
}

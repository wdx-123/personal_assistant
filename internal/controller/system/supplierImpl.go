package system

type controllerSupplier struct {
	refreshTokenCtrl *RefreshTokenCtrl
	baseCtrl         *BaseCtrl
	userCtrl         *UserCtrl
	orgCtrl          *OrgCtrl
	ojCtrl           *OJCtrl
	apiCtrl          *ApiCtrl
	menuCtrl         *MenuCtrl
	roleCtrl         *RoleCtrl
	imageCtrl        *ImageCtrl
}

func (c *controllerSupplier) GetRefreshTokenCtrl() *RefreshTokenCtrl {
	return c.refreshTokenCtrl
}
func (c *controllerSupplier) GetBaseCtrl() *BaseCtrl {
	return c.baseCtrl
}
func (c *controllerSupplier) GetUserCtrl() *UserCtrl {
	return c.userCtrl
}
func (c *controllerSupplier) GetOrgCtrl() *OrgCtrl {
	return c.orgCtrl
}

func (c *controllerSupplier) GetOJCtrl() *OJCtrl {
	return c.ojCtrl
}

func (c *controllerSupplier) GetApiCtrl() *ApiCtrl {
	return c.apiCtrl
}

func (c *controllerSupplier) GetMenuCtrl() *MenuCtrl {
	return c.menuCtrl
}

func (c *controllerSupplier) GetRoleCtrl() *RoleCtrl {
	return c.roleCtrl
}

func (c *controllerSupplier) GetImageCtrl() *ImageCtrl {
	return c.imageCtrl
}

package system

type controllerSupplier struct {
	refreshTokenCtrl *RefreshTokenCtrl
	baseCtrl         *BaseCtrl
	userCtrl         *UserCtrl
	orgCtrl          *OrgCtrl
	ojCtrl           *OJCtrl
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

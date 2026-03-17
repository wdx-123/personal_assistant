package system

type controllerSupplier struct {
	refreshTokenCtrl  *RefreshTokenCtrl
	baseCtrl          *BaseCtrl
	healthCtrl        *HealthCtrl
	userCtrl          *UserCtrl
	orgCtrl           *OrgCtrl
	ojCtrl            *OJCtrl
	ojTaskCtrl        *OJTaskCtrl
	apiCtrl           *ApiCtrl
	menuCtrl          *MenuCtrl
	roleCtrl          *RoleCtrl
	imageCtrl         *ImageCtrl
	observabilityCtrl *ObservabilityCtrl
}

func (c *controllerSupplier) GetRefreshTokenCtrl() *RefreshTokenCtrl {
	return c.refreshTokenCtrl
}
func (c *controllerSupplier) GetBaseCtrl() *BaseCtrl {
	return c.baseCtrl
}
func (c *controllerSupplier) GetHealthCtrl() *HealthCtrl {
	return c.healthCtrl
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

func (c *controllerSupplier) GetOJTaskCtrl() *OJTaskCtrl {
	return c.ojTaskCtrl
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

func (c *controllerSupplier) GetObservabilityCtrl() *ObservabilityCtrl {
	return c.observabilityCtrl
}

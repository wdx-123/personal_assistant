package system

import (
	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	serviceSystem "personal_assistant/internal/service/system"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type OrgCtrl struct {
	orgService *serviceSystem.OrgService
}

func NewOrgCtrl(orgService *serviceSystem.OrgService) *OrgCtrl {
	return &OrgCtrl{
		orgService: orgService,
	}
}

// GetOrgList 获取组织列表（支持分页与不分页）
func (ctrl *OrgCtrl) GetOrgList(c *gin.Context) {
	var req request.OrgListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		global.Log.Error("参数绑定失败", zap.Error(err))
		response.NewResponse[any, any](c).
			SetCode(global.StatusBadRequest).
			Failed("参数错误", nil)
		return
	}

	// 默认值处理：如果只传了pageSize没传page，默认第一页；如果什么都没传，默认不分页
	if req.PageSize > 0 && req.Page == 0 {
		req.Page = 1
	}

	// 调用服务层
	// Service层约定：page <= 0 时查询所有
	orgs, total, err := ctrl.orgService.GetOrgList(c.Request.Context(), req.Page, req.PageSize)
	if err != nil {
		global.Log.Error("获取组织列表失败", zap.Error(err))
		response.NewResponse[any, any](c).
			SetCode(global.StatusInternalServerError).
			Failed("获取组织列表失败", nil)
		return
	}

	// 组装响应 DTO
	respData := &resp.OrgListResp{
		List:  orgs,
		Total: total,
	}

	response.NewResponse[resp.OrgListResp, resp.OrgListResp](c).
		SetCode(global.StatusOK).
		Success("获取成功", respData)
}

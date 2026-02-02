package system

import (
	"strconv"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	serviceSystem "personal_assistant/internal/service/system"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ApiCtrl API接口管理控制器
type ApiCtrl struct {
	apiService *serviceSystem.ApiService
}

// NewApiCtrl 创建API控制器实例
func NewApiCtrl(apiService *serviceSystem.ApiService) *ApiCtrl {
	return &ApiCtrl{apiService: apiService}
}

// GetAPIList 获取API列表
func (c *ApiCtrl) GetAPIList(ctx *gin.Context) {
	var req request.ApiListReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		global.Log.Error("API列表参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", ctx)
		return
	}

	filter := &request.ApiListFilter{
		Page:     req.Page,
		PageSize: req.PageSize,
		Status:   req.Status,
		GroupID:  req.GroupID,
		Method:   req.Method,
		Keyword:  req.Keyword,
	}

	list, total, err := c.apiService.GetAPIList(ctx.Request.Context(), filter)
	if err != nil {
		global.Log.Error("获取API列表失败", zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}

	items := make([]*resp.ApiItem, 0, len(list))
	for _, api := range list {
		items = append(items, entityToApiItem(api))
	}
	response.BizOkWithPage(items, total, filter.Page, filter.PageSize, ctx)
}

// GetAPIByID 获取API详情
func (c *ApiCtrl) GetAPIByID(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BizFailWithMessage("ID格式错误", ctx)
		return
	}
	api, err := c.apiService.GetAPIByID(ctx.Request.Context(), uint(id))
	if err != nil {
		global.Log.Error("获取API详情失败", zap.Uint64("id", id), zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithData(entityToApiItem(api), ctx)
}

// CreateAPI 创建API
func (c *ApiCtrl) CreateAPI(ctx *gin.Context) {
	var req request.CreateApiReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		global.Log.Error("创建API参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", ctx)
		return
	}
	api := &entity.API{
		Path:    req.Path,
		Method:  req.Method,
		Detail:  req.Detail,
		GroupID: req.GroupID,
		Status:  req.Status,
	}
	if api.Status == 0 {
		api.Status = 1
	}
	if err := c.apiService.CreateAPI(ctx.Request.Context(), api); err != nil {
		global.Log.Error("创建API失败", zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithMessage("创建成功", ctx)
}

// UpdateAPI 更新API
func (c *ApiCtrl) UpdateAPI(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BizFailWithMessage("ID格式错误", ctx)
		return
	}
	var req request.UpdateApiReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		global.Log.Error("更新API参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", ctx)
		return
	}
	if err := c.apiService.UpdateAPI(ctx.Request.Context(), uint(id), req.Path, req.Method, req.Detail, req.GroupID, req.Status); err != nil {
		global.Log.Error("更新API失败", zap.Uint64("id", id), zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithMessage("更新成功", ctx)
}

// DeleteAPI 删除API
func (c *ApiCtrl) DeleteAPI(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BizFailWithMessage("ID格式错误", ctx)
		return
	}
	if err := c.apiService.DeleteAPI(ctx.Request.Context(), uint(id)); err != nil {
		global.Log.Error("删除API失败", zap.Uint64("id", id), zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithMessage("删除成功", ctx)
}

// SyncAPI 同步路由到API表
func (c *ApiCtrl) SyncAPI(ctx *gin.Context) {
	var req request.SyncApiReq
	_ = ctx.ShouldBindJSON(&req)
	added, updated, disabled, total, err := c.apiService.SyncAPI(ctx.Request.Context(), req.DeleteRemoved)
	if err != nil {
		global.Log.Error("同步API失败", zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithDetailed(&resp.SyncApiResp{
		Added:    added,
		Updated:  updated,
		Disabled: disabled,
		Total:    total,
	}, "同步成功", ctx)
}

// entityToApiItem 将 entity.API 转为 response.ApiItem
func entityToApiItem(api *entity.API) *resp.ApiItem {
	if api == nil {
		return nil
	}
	return &resp.ApiItem{
		ID:        api.ID,
		Path:      api.Path,
		Method:    api.Method,
		Detail:    api.Detail,
		GroupID:   api.GroupID,
		Status:    api.Status,
		CreatedAt: api.CreatedAt,
		UpdatedAt: api.UpdatedAt,
	}
}

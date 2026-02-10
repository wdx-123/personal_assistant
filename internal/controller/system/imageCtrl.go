/**
 * @description: 图片管理控制器，处理图片上传、删除、查询请求
 *               仅负责参数校验与响应组装，业务逻辑委托给 ImageService
 */
package system

import (
	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	serviceSystem "personal_assistant/internal/service/system"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ImageCtrl 图片管理控制器
type ImageCtrl struct {
	imageService *serviceSystem.ImageService
}

// NewImageCtrl 创建图片控制器实例
func NewImageCtrl(imageService *serviceSystem.ImageService) *ImageCtrl {
	return &ImageCtrl{imageService: imageService}
}

// Upload 上传图片（使用当前驱动）
// @Summary 上传图片（支持多文件）
// @Tags System: Image
// @Accept multipart/form-data
// @Produce json
// @Param files formData file true "图片文件（可多选）"
// @Param category formData int false "图片分类" default(0)
// @Param driver formData string false "指定存储驱动（local/qiniu），为空则使用当前配置"
// @Success 200 {object} response.BizResponse
// @Router /api/system/image/upload [post]
func (ctrl *ImageCtrl) Upload(c *gin.Context) {
	// 1. 解析表单参数
	var req request.UploadImageReq
	if err := c.ShouldBind(&req); err != nil {
		global.Log.Error("图片上传参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	// 2. 获取上传文件
	form, err := c.MultipartForm()
	if err != nil {
		global.Log.Error("解析上传表单失败", zap.Error(err))
		response.BizFailWithMessage("请选择要上传的文件", c)
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		response.BizFailWithMessage("请选择要上传的文件", c)
		return
	}

	// 3. 提取用户 ID（必须成功，否则产生无主图片）
	uploaderID := jwt.GetUserID(c)
	if uploaderID == 0 {
		global.Log.Error("提取用户ID失败：UserID 为 0 或 claims 不存在")
		response.BizFailWithMessage("无法获取用户信息，请重新登录", c)
		return
	}

	// 4. 调用 Service 上传（req.Driver 为空时自动使用默认驱动）
	items, err := ctrl.imageService.Upload(c.Request.Context(), files, &req, uploaderID)
	if err != nil {
		global.Log.Error("图片上传失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}

	response.BizOkWithData(items, c)
}

// Delete 批量删除图片
// @Summary 批量删除图片
// @Tags System: Image
// @Accept json
// @Produce json
// @Param body body request.DeleteImageReq true "删除请求"
// @Success 200 {object} response.BizResponse
// @Router /api/system/image/delete [delete]
func (ctrl *ImageCtrl) Delete(c *gin.Context) {
	var req request.DeleteImageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("图片删除参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	if err := ctrl.imageService.Delete(c.Request.Context(), req.IDs); err != nil {
		global.Log.Error("图片删除失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}

	response.BizOk(c)
}

// List 图片列表（分页）
// @Summary 获取图片列表
// @Tags System: Image
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param category query int false "图片分类过滤"
// @Success 200 {object} response.BizResponse
// @Router /api/system/image/list [get]
func (ctrl *ImageCtrl) List(c *gin.Context) {
	var req request.ListImageReq
	if err := c.ShouldBindQuery(&req); err != nil {
		global.Log.Error("图片列表参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	items, total, err := ctrl.imageService.List(c.Request.Context(), &req)
	if err != nil {
		global.Log.Error("获取图片列表失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}

	response.BizOkWithPage(items, total, req.Page, req.PageSize, c)
}



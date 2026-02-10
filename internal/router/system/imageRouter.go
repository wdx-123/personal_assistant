/**
 * @description: 图片管理路由，注册图片上传、删除、列表查询接口
 */
package system

import (
	"personal_assistant/internal/controller"

	"github.com/gin-gonic/gin"
)

// ImageRouter 图片管理路由
type ImageRouter struct{}

// InitImageRouter 初始化图片路由，挂载到 SystemGroup（需JWT+权限）
// uploadRateLimitMW: 上传接口限流中间件（仅作用于 upload 路由，不影响 delete/list）
func (r *ImageRouter) InitImageRouter(
	router *gin.RouterGroup,
	uploadRateLimitMW gin.HandlerFunc,
) {
	imageGroup := router.Group("api/system/image")
	imageCtrl := controller.ApiGroupApp.SystemApiGroup.GetImageCtrl()
	{
		imageGroup.POST("upload", uploadRateLimitMW, imageCtrl.Upload) // 上传图片（带限流）
		imageGroup.DELETE("delete", imageCtrl.Delete)                  // 批量删除图片
		imageGroup.GET("list", imageCtrl.List)                         // 图片列表
	}
}

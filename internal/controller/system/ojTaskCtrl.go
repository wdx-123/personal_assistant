package system

import (
	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	serviceContract "personal_assistant/internal/service/contract"
	bizerrors "personal_assistant/pkg/errors"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/response"
	"personal_assistant/pkg/util"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type OJTaskCtrl struct {
	ojTaskService serviceContract.OJTaskServiceContract
}

// AnalyzeTaskTitles 分析 OJTask 题目标题。
func (ctrl *OJTaskCtrl) AnalyzeTaskTitles(c *gin.Context) {
	var req request.AnalyzeOJTaskTitlesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("分析 OJTask 题目参数错误", zap.Error(err))
		response.BizFailWithCodeMsg(bizerrors.CodeBindFailed, err.Error(), c)
		return
	}

	out, err := ctrl.ojTaskService.AnalyzeTaskTitles(c.Request.Context(), &req)
	if err != nil {
		global.Log.Error("分析 OJTask 题目失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(out, c)                          
}

// CreateTask 创建 OJTask
func (ctrl *OJTaskCtrl) CreateTask(c *gin.Context) {
	var req request.CreateOJTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("创建 OJTask 参数错误", zap.Error(err))
		response.BizFailWithCodeMsg(bizerrors.CodeBindFailed, err.Error(), c)
		return
	}

	userID := jwt.GetUserID(c)
	out, err := ctrl.ojTaskService.CreateTask(c.Request.Context(), userID, &req)
	if err != nil {
		global.Log.Error("创建 OJTask 失败", zap.Uint("user_id", userID), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(out, c)
}

// UpdateTask 更新 OJTask
func (ctrl *OJTaskCtrl) UpdateTask(c *gin.Context) {
	var req request.UpdateOJTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("更新 OJTask 参数错误", zap.Error(err))
		response.BizFailWithCodeMsg(bizerrors.CodeBindFailed, err.Error(), c)
		return
	}

	taskID := util.ParseUint(c.Param("id"))
	if taskID == 0 {
		response.BizFailWithCode(bizerrors.CodeInvalidParams, c)
		return
	}

	userID := jwt.GetUserID(c)
	if err := ctrl.ojTaskService.UpdateTask(c.Request.Context(), userID, taskID, &req); err != nil {
		global.Log.Error("更新 OJTask 失败", zap.Uint("user_id", userID), zap.Uint("task_id", taskID), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOk(c)
}

// DeleteTask 删除 OJTask
func (ctrl *OJTaskCtrl) DeleteTask(c *gin.Context) {
	taskID := util.ParseUint(c.Param("id"))
	if taskID == 0 {
		response.BizFailWithCode(bizerrors.CodeInvalidParams, c)
		return
	}

	userID := jwt.GetUserID(c)
	if err := ctrl.ojTaskService.DeleteTask(c.Request.Context(), userID, taskID); err != nil {
		global.Log.Error("删除 OJTask 失败", zap.Uint("user_id", userID), zap.Uint("task_id", taskID), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOk(c)
}

// DeleteTask 删除 OJTask
func (ctrl *OJTaskCtrl) ExecuteTaskNow(c *gin.Context) {
	taskID := util.ParseUint(c.Param("id"))
	if taskID == 0 {
		response.BizFailWithCode(bizerrors.CodeInvalidParams, c)
		return
	}

	userID := jwt.GetUserID(c)
	out, err := ctrl.ojTaskService.ExecuteTaskNow(c.Request.Context(), userID, taskID)
	if err != nil {
		global.Log.Error("提前执行 OJTask 失败", zap.Uint("user_id", userID), zap.Uint("task_id", taskID), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(out, c)
}

// ReviseTask 派生 OJTask
func (ctrl *OJTaskCtrl) ReviseTask(c *gin.Context) {
	var req request.ReviseOJTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("派生 OJTask 参数错误", zap.Error(err))
		response.BizFailWithCodeMsg(bizerrors.CodeBindFailed, err.Error(), c)
		return
	}

	// taskID 从路径参数获取，确保用户只能派生自己的任务
	taskID := util.ParseUint(c.Param("id"))
	if taskID == 0 {
		response.BizFailWithCode(bizerrors.CodeInvalidParams, c)
		return
	}

	userID := jwt.GetUserID(c)
	out, err := ctrl.ojTaskService.ReviseTask(c.Request.Context(), userID, taskID, &req)
	if err != nil {
		global.Log.Error("派生 OJTask 失败", zap.Uint("user_id", userID), zap.Uint("task_id", taskID), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(out, c)
}

// RetryTask 重试 OJTask
func (ctrl *OJTaskCtrl) RetryTask(c *gin.Context) {
	taskID := util.ParseUint(c.Param("id"))
	if taskID == 0 {
		response.BizFailWithCode(bizerrors.CodeInvalidParams, c)
		return
	}

	userID := jwt.GetUserID(c)
	out, err := ctrl.ojTaskService.RetryTask(c.Request.Context(), userID, taskID)
	if err != nil {
		global.Log.Error("重试 OJTask 失败", zap.Uint("user_id", userID), zap.Uint("task_id", taskID), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(out, c)
}

// GetVisibleTaskList 获取可见的 OJTask 列表
func (ctrl *OJTaskCtrl) GetVisibleTaskList(c *gin.Context) {
	var req request.OJTaskListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		global.Log.Error("查询 OJTask 列表参数错误", zap.Error(err))
		response.BizFailWithCodeMsg(bizerrors.CodeBindFailed, err.Error(), c)
		return
	}

	userID := jwt.GetUserID(c)
	list, total, err := ctrl.ojTaskService.GetVisibleTaskList(c.Request.Context(), userID, &req)
	if err != nil {
		global.Log.Error("查询 OJTask 列表失败", zap.Uint("user_id", userID), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	response.BizOkWithPage(list, total, page, pageSize, c)
}

// GetTaskDetail 获取 OJTask 详情
func (ctrl *OJTaskCtrl) GetTaskDetail(c *gin.Context) {
	taskID := util.ParseUint(c.Param("id"))
	if taskID == 0 {
		response.BizFailWithCode(bizerrors.CodeInvalidParams, c)
		return
	}

	userID := jwt.GetUserID(c)
	out, err := ctrl.ojTaskService.GetTaskDetail(c.Request.Context(), userID, taskID)
	if err != nil {
		global.Log.Error("查询 OJTask 详情失败", zap.Uint("user_id", userID), zap.Uint("task_id", taskID), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(out, c)
}

// GetTaskVersions 获取 OJTask 版本列表
func (ctrl *OJTaskCtrl) GetTaskVersions(c *gin.Context) {
	taskID := util.ParseUint(c.Param("id"))
	if taskID == 0 {
		response.BizFailWithCode(bizerrors.CodeInvalidParams, c)
		return
	}

	userID := jwt.GetUserID(c)
	out, err := ctrl.ojTaskService.GetTaskVersions(c.Request.Context(), userID, taskID)
	if err != nil {
		global.Log.Error("查询 OJTask 版本失败", zap.Uint("user_id", userID), zap.Uint("task_id", taskID), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(out, c)
}

// GetTaskExecutionDetail 获取 OJTask 执行详情
func (ctrl *OJTaskCtrl) GetTaskExecutionDetail(c *gin.Context) {
	taskID := util.ParseUint(c.Param("id"))
	executionID := util.ParseUint(c.Param("executionId"))
	if taskID == 0 || executionID == 0 {
		response.BizFailWithCode(bizerrors.CodeInvalidParams, c)
		return
	}

	userID := jwt.GetUserID(c)
	out, err := ctrl.ojTaskService.GetTaskExecutionDetail(c.Request.Context(), userID, taskID, executionID)
	if err != nil {
		global.Log.Error(
			"查询 OJTask 执行详情失败",
			zap.Uint("user_id", userID),
			zap.Uint("task_id", taskID),
			zap.Uint("execution_id", executionID),
			zap.Error(err),
		)
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(out, c)
}

// GetTaskExecutionUsers 获取 OJTask 执行用户列表
func (ctrl *OJTaskCtrl) GetTaskExecutionUsers(c *gin.Context) {
	var req request.OJTaskExecutionUserListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		global.Log.Error("查询 OJTask 执行用户参数错误", zap.Error(err))
		response.BizFailWithCodeMsg(bizerrors.CodeBindFailed, err.Error(), c)
		return
	}

	taskID := util.ParseUint(c.Param("id"))
	executionID := util.ParseUint(c.Param("executionId"))
	if taskID == 0 || executionID == 0 {
		response.BizFailWithCode(bizerrors.CodeInvalidParams, c)
		return
	}

	userID := jwt.GetUserID(c)
	out, err := ctrl.ojTaskService.GetTaskExecutionUsers(c.Request.Context(), userID, taskID, executionID, &req)
	if err != nil {
		global.Log.Error(
			"查询 OJTask 执行用户失败",
			zap.Uint("user_id", userID),
			zap.Uint("task_id", taskID),
			zap.Uint("execution_id", executionID),
			zap.Error(err),
		)
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithPage(out.List, out.Total, out.Page, out.PageSize, c)
}

// GetTaskExecutionUserDetail 获取 OJTask 执行用户详情
func (ctrl *OJTaskCtrl) GetTaskExecutionUserDetail(c *gin.Context) {
	taskID := util.ParseUint(c.Param("id"))
	executionID := util.ParseUint(c.Param("executionId"))
	targetUserID := util.ParseUint(c.Param("userId"))
	if taskID == 0 || executionID == 0 || targetUserID == 0 {
		response.BizFailWithCode(bizerrors.CodeInvalidParams, c)
		return
	}

	userID := jwt.GetUserID(c)
	out, err := ctrl.ojTaskService.GetTaskExecutionUserDetail(
		c.Request.Context(),
		userID,
		taskID,
		executionID,
		targetUserID,
	)
	if err != nil {
		global.Log.Error(
			"查询 OJTask 执行用户详情失败",
			zap.Uint("user_id", userID),
			zap.Uint("task_id", taskID),
			zap.Uint("execution_id", executionID),
			zap.Uint("target_user_id", targetUserID),
			zap.Error(err),
		)
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(out, c)
}

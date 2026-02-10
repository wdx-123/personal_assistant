/**
 * @description: 图片管理服务，提供图片上传、删除、查询功能
 *               编排存储驱动（pkg/storage）与仓储层（repository），不直接操作 DB 或文件系统
 */
package system

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/errors"
	"personal_assistant/pkg/storage"
	"personal_assistant/pkg/util"

	"go.uber.org/zap"
)

// ImageService 图片管理服务
type ImageService struct {
	imageRepo   interfaces.ImageRepository
	uploadSem   chan struct{}    // 并发上传信号量，控制同时进行的上传数量
	allowedMIME map[string]bool // 缓存的 MIME 白名单，启动时构建，运行期只读
}

// NewImageService 创建图片服务实例
// 信号量容量由 static.max_concurrent_uploads 配置驱动，为 0 时不限制并发
func NewImageService(repositoryGroup *repository.Group) *ImageService {
	maxConcurrent := global.Config.Static.MaxConcurrentUploads
	if maxConcurrent <= 0 {
		maxConcurrent = 50 // 零值兜底
	}
	return &ImageService{
		imageRepo:   repositoryGroup.SystemRepositorySupplier.GetImageRepository(),
		uploadSem:   make(chan struct{}, maxConcurrent),
		allowedMIME: buildAllowedMIMETypes(),
	}
}

// ==================== 上传 ====================

// Upload 上传图片：校验 → 计算哈希 → 驱动上传 → 入库
// 驱动选择：req.Driver 不为空时使用指定驱动，否则使用当前配置的默认驱动
func (s *ImageService) Upload(
	ctx context.Context,
	files []*multipart.FileHeader,
	req *request.UploadImageReq,
	uploaderID uint,
) ([]response.ImageItem, error) {
	drv := s.resolveDriverByName(req.Driver)
	if drv == nil {
		return nil, errors.NewWithMsg(errors.CodeInternalError, "存储驱动未初始化")
	}
	return s.uploadWithDriver(ctx, drv, files, req.Category, uploaderID)
}

// SaveGeneratedImage 后端直接存图并入库（用于程序生成的图片，如验证码、图表等）
// 参数：filename 文件名（不含扩展名）、imageType 扩展名（如 ".png"）、imgBytes 图片二进制数据
// 返回：图片 ID、访问 URL
func (s *ImageService) SaveGeneratedImage(
	ctx context.Context,
	filename, imageType string,
	imgBytes []byte,
	category consts.Category,
	uploaderID uint,
) (uint, string, error) {
	// 大小限制校验
	maxSize := int64(global.Config.Static.MaxSize) << 20
	if maxSize > 0 && int64(len(imgBytes)) > maxSize {
		return 0, "", errors.NewWithMsg(errors.CodeInvalidParams,
			fmt.Sprintf("生成图片大小超过限制（最大 %dMB）", global.Config.Static.MaxSize))
	}

	drv := storage.CurrentDriver()
	if drv == nil {
		return 0, "", errors.NewWithMsg(errors.CodeInternalError, "存储驱动未初始化")
	}

	// 获取并发信号量（内部调用同样占用 I/O 资源）
	if err := s.acquireSemaphore(ctx); err != nil {
		return 0, "", err
	}
	defer s.releaseSemaphore()

	// 超时控制
	uploadCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	fullName := filename + imageType
	r := bytes.NewReader(imgBytes)

	// 上传
	obj, err := drv.Upload(uploadCtx, r, fullName)
	if err != nil {
		return 0, "", errors.WrapWithMsg(errors.CodeInternalError, "文件上传失败", err)
	}

	// 计算文件内容哈希
	fileHash := util.FileHashBytes(imgBytes)

	// 入库
	img := &entity.Image{
		Name:       fullName,
		Type:       strings.ToLower(imageType),
		Size:       int64(len(imgBytes)),
		Driver:     drv.Name(),
		Key:        obj.Key,
		URL:        obj.URL,
		Category:   category,
		UploaderID: uploaderID,
		FileHash:   fileHash,
		HashAlgo:   util.FileHashAlgo,
	}
	if err := s.imageRepo.Create(ctx, img); err != nil {
		return 0, "", errors.Wrap(errors.CodeDBError, err)
	}

	return img.ID, img.URL, nil
}

// ==================== 删除 ====================

// Delete 软删除图片记录，物理文件由定时任务 CleanOrphanFiles 异步清理
func (s *ImageService) Delete(ctx context.Context, ids []uint) error {
	if err := s.imageRepo.Delete(ctx, ids); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}

// CleanOrphanFiles 清理孤儿物理文件（由定时任务调用）
// 流程：查找孤儿 key → 逐个删除物理文件 → 清理已软删除的 DB 记录
func (s *ImageService) CleanOrphanFiles(ctx context.Context) error {
	keys, drivers, err := s.imageRepo.FindOrphanKeys(ctx)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if len(keys) == 0 {
		return nil
	}

	// 逐个删除物理文件，收集成功的 key
	successKeys := make([]string, 0, len(keys))
	for i, key := range keys {
		drv := storage.DriverFromName(drivers[i])
		if drv == nil {
			drv = storage.CurrentDriver()
		}
		if drv == nil {
			global.Log.Warn("清理孤儿文件跳过：无可用存储驱动",
				zap.String("key", key), zap.String("driver", drivers[i]))
			continue
		}
		if delErr := drv.Delete(ctx, key); delErr != nil {
			global.Log.Warn("清理孤儿物理文件失败",
				zap.String("driver", drv.Name()),
				zap.String("key", key),
				zap.Error(delErr))
			continue
		}
		successKeys = append(successKeys, key)
	}

	// 物理文件删除成功后，清理对应的软删除 DB 记录
	if len(successKeys) > 0 {
		if err := s.imageRepo.HardDeleteByKeys(ctx, successKeys); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
	}
	return nil
}

// ==================== 查询 ====================

// List 分页查询图片列表
func (s *ImageService) List(ctx context.Context, req *request.ListImageReq) ([]response.ImageItem, int64, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 10
	}
	offset := (req.Page - 1) * req.PageSize

	images, total, err := s.imageRepo.List(ctx, req.Category, offset, req.PageSize)
	if err != nil {
		return nil, 0, errors.Wrap(errors.CodeDBError, err)
	}

	items := make([]response.ImageItem, len(images))
	for i, img := range images {
		items[i] = response.ImageItem{
			ID:            img.ID,
			URL:           img.URL,
			Name:          img.Name,
			Type:          img.Type,
			Size:          img.Size,
			Category:      img.Category,
			CategoryLabel: img.Category.String(),
		}
	}
	return items, total, nil
}

// GetURL 获取单张图片的访问 URL
func (s *ImageService) GetURL(ctx context.Context, id uint) (string, error) {
	img, err := s.imageRepo.GetByID(ctx, id)
	if err != nil {
		return "", errors.Wrap(errors.CodeDBError, err)
	}
	if img == nil {
		return "", errors.New(errors.CodeInvalidParams)
	}
	return img.URL, nil
}

// ==================== 内部方法 ====================

// uploadWithDriver 使用指定驱动处理多文件上传
// 执行流程：检查文件数量 → 检查配额 → 逐个上传（信号量在单文件级别控制）
func (s *ImageService) uploadWithDriver(
	ctx context.Context,
	drv storage.Driver,
	files []*multipart.FileHeader,
	category consts.Category,
	uploaderID uint,
) ([]response.ImageItem, error) {
	maxUploads := global.Config.Static.MaxUploads
	if maxUploads > 0 && len(files) > maxUploads {
		return nil, errors.NewWithMsg(errors.CodeInvalidParams,
			fmt.Sprintf("单次最多上传 %d 张图片", maxUploads))
	}

	// 配额检查：用 FileHeader.Size 之和作为预估值（上传前可得）
	// 注意：并发场景下配额检查存在 TOCTOU 窗口，信号量+限流已约束并发，超额风险可控
	var incomingSize int64
	for _, fh := range files {
		incomingSize += fh.Size
	}
	if err := s.checkQuota(ctx, uploaderID, incomingSize); err != nil {
		return nil, err
	}

	// 逐个上传（信号量在 uploadSingle 内获取，确保按文件公平调度）
	items := make([]response.ImageItem, 0, len(files))
	for _, fh := range files {
		item, err := s.uploadSingle(ctx, drv, fh, category, uploaderID)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, nil
}

// uploadSingle 处理单个文件的上传流程
// 流程：获取信号量 → 超时控制 → 校验扩展名/大小 → 打开文件 → Magic Bytes 内容校验 → 计算哈希 → 秒传查库 → 驱动上传 → 入库
func (s *ImageService) uploadSingle(
	ctx context.Context,
	drv storage.Driver,
	fh *multipart.FileHeader,
	category consts.Category,
	uploaderID uint,
) (*response.ImageItem, error) {
	// 0. 获取并发信号量（按单文件公平调度）
	if err := s.acquireSemaphore(ctx); err != nil {
		return nil, err
	}
	defer s.releaseSemaphore()

	// 0b. 单文件上传超时控制
	uploadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 1. 校验文件扩展名与大小
	if err := s.validateFile(fh); err != nil {
		return nil, err
	}

	// 2. 打开文件流（multipart.File 实现了 io.ReadSeeker）
	src, err := fh.Open()
	if err != nil {
		return nil, errors.Wrap(errors.CodeInternalError, err)
	}
	defer func() { _ = src.Close() }()

	// 3. Magic Bytes 内容校验：读取文件头判断真实类型，防止扩展名伪造
	if err := s.validateContentType(src); err != nil {
		return nil, err
	}

	// 4. 完整读取文件计算哈希（秒传需要先知道哈希再决定是否上传）
	fileHash, err := util.FileHashReader(src)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInternalError, err)
	}

	// 5. 秒传：查库是否已有相同哈希+大小的文件
	existing, err := s.imageRepo.GetByFileHash(uploadCtx, fileHash, fh.Size)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	if existing != nil {
		// 命中秒传：创建新 DB 记录，复用已有文件的 Key/URL/Driver
		return s.createInstantUploadRecord(uploadCtx, existing, fh, category, uploaderID, fileHash)
	}

	// 6. 未命中：Seek 回文件开头，流式上传到存储驱动
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, errors.Wrap(errors.CodeInternalError, err)
	}
	obj, err := drv.Upload(uploadCtx, src, fh.Filename)
	if err != nil {
		return nil, errors.WrapWithMsg(errors.CodeInternalError, "文件上传失败", err)
	}

	// 7. 构建实体并入库
	img := &entity.Image{
		Name:       fh.Filename,
		Type:       strings.ToLower(filepath.Ext(fh.Filename)),
		Size:       obj.Size,
		Driver:     drv.Name(),
		Key:        obj.Key,
		URL:        obj.URL,
		Category:   category,
		UploaderID: uploaderID,
		FileHash:   fileHash,
		HashAlgo:   util.FileHashAlgo,
	}
	if err := s.imageRepo.Create(uploadCtx, img); err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}

	return s.toImageItem(img), nil
}

// createInstantUploadRecord 秒传命中：创建新 DB 记录，复用已有文件的存储位置
// 新记录有独立的 ID、uploaderID、category，但 Key/URL/Driver 与已有文件相同
func (s *ImageService) createInstantUploadRecord(
	ctx context.Context,
	existing *entity.Image,
	fh *multipart.FileHeader,
	category consts.Category,
	uploaderID uint,
	fileHash string,
) (*response.ImageItem, error) {
	img := &entity.Image{
		Name:       fh.Filename,
		Type:       strings.ToLower(filepath.Ext(fh.Filename)),
		Size:       existing.Size,
		Driver:     existing.Driver,
		Key:        existing.Key,
		URL:        existing.URL,
		Category:   category,
		UploaderID: uploaderID,
		FileHash:   fileHash,
		HashAlgo:   util.FileHashAlgo,
	}
	if err := s.imageRepo.Create(ctx, img); err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	return s.toImageItem(img), nil
}

// toImageItem 将 entity 转换为响应 DTO
func (s *ImageService) toImageItem(img *entity.Image) *response.ImageItem {
	return &response.ImageItem{
		ID:            img.ID,
		URL:           img.URL,
		Name:          img.Name,
		Type:          img.Type,
		Size:          img.Size,
		Category:      img.Category,
		CategoryLabel: img.Category.String(),
	}
}

// validateFile 校验文件类型与大小
func (s *ImageService) validateFile(fh *multipart.FileHeader) error {
	maxSize := int64(global.Config.Static.MaxSize) << 20
	if maxSize > 0 && fh.Size > maxSize {
		return errors.NewWithMsg(errors.CodeInvalidParams,
			fmt.Sprintf("文件大小超过限制（最大 %dMB）", global.Config.Static.MaxSize))
	}

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	allowed := global.Config.Static.AllowedTypes
	if len(allowed) > 0 {
		valid := false
		for _, t := range allowed {
			if strings.EqualFold(ext, t) {
				valid = true
				break
			}
		}
		if !valid {
			return errors.NewWithMsg(errors.CodeInvalidParams,
				fmt.Sprintf("不支持的文件类型: %s", ext))
		}
	}
	return nil
}

// validateContentType 读取文件头 512 字节校验真实 MIME 类型，防止扩展名伪造
// 校验完成后自动 Seek 回文件开头，不影响后续读取
func (s *ImageService) validateContentType(src multipart.File) error {
	buf := make([]byte, 512)
	n, err := src.Read(buf)
	if err != nil && err != io.EOF {
		return errors.Wrap(errors.CodeInternalError, err)
	}
	contentType := http.DetectContentType(buf[:n])
	if !s.allowedMIME[contentType] {
		return errors.NewWithMsg(errors.CodeInvalidParams,
			fmt.Sprintf("文件内容类型不合法: %s", contentType))
	}
	// Seek 回文件开头
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return errors.Wrap(errors.CodeInternalError, err)
	}
	return nil
}

// buildAllowedMIMETypes 从配置的扩展名白名单自动推导出 MIME 类型白名单
// 扩展名 → MIME 类型的映射由标准库 mime.TypeByExtension 完成
// 未能识别的扩展名（如 .bin）兜底为 application/octet-stream
func buildAllowedMIMETypes() map[string]bool {
	result := make(map[string]bool)
	for _, ext := range global.Config.Static.AllowedTypes {
		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			// 标准库无法识别的扩展名（如 .bin），兜底为通用二进制类型
			mimeType = "application/octet-stream"
		}
		// mime.TypeByExtension 可能返回带参数的类型（如 "text/plain; charset=utf-8"），只取主类型
		if idx := strings.Index(mimeType, ";"); idx != -1 {
			mimeType = strings.TrimSpace(mimeType[:idx])
		}
		result[mimeType] = true
	}
	return result
}

// resolveDriverByName 根据驱动名获取驱动实例，为空或不存在时回退到当前驱动
func (s *ImageService) resolveDriverByName(name string) storage.Driver {
	if name != "" {
		if drv := storage.DriverFromName(name); drv != nil {
			return drv
		}
	}
	return storage.CurrentDriver()
}

// ==================== 防护：信号量 & 配额 ====================

// acquireSemaphore 获取上传并发信号量，Context 取消时快速返回错误
func (s *ImageService) acquireSemaphore(ctx context.Context) error {
	select {
	case s.uploadSem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return errors.NewWithMsg(errors.CodeTooManyRequests, "服务器繁忙，请稍后重试")
	}
}

// releaseSemaphore 释放上传并发信号量
func (s *ImageService) releaseSemaphore() {
	<-s.uploadSem
}

// checkQuota 检查用户存储配额
// quotaBytes <= 0 表示不限制，incomingSize 为本次待上传文件的预估总大小
func (s *ImageService) checkQuota(ctx context.Context, uploaderID uint, incomingSize int64) error {
	quotaBytes := int64(global.Config.Static.UserQuotaMB) << 20
	if quotaBytes <= 0 {
		return nil // 未配置配额，不限制
	}
	usedBytes, err := s.imageRepo.SumSizeByUploader(ctx, uploaderID)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if usedBytes+incomingSize > quotaBytes {
		return errors.NewWithMsg(errors.CodeInvalidParams,
			fmt.Sprintf("存储空间不足，已用 %dMB / 配额 %dMB",
				usedBytes>>20, global.Config.Static.UserQuotaMB))
	}
	return nil
}

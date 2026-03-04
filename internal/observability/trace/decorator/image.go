package decorator

import (
	"context"
	"mime/multipart"

	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/service/contract"
)

type tracedImageService struct {
	next contract.ImageServiceContract
}

func WrapImageService(next contract.ImageServiceContract) contract.ImageServiceContract {
	if next == nil {
		return nil
	}
	return &tracedImageService{next: next}
}

func (t *tracedImageService) Upload(
	ctx context.Context,
	files []*multipart.FileHeader,
	req *request.UploadImageReq,
	uploaderID uint,
) ([]resp.ImageItem, error) {
	return runTraced(ctx, "image", "Upload", func(inner context.Context) ([]resp.ImageItem, error) {
		return t.next.Upload(inner, files, req, uploaderID)
	})
}

func (t *tracedImageService) Delete(ctx context.Context, ids []uint) error {
	return runTracedErr(ctx, "image", "Delete", func(inner context.Context) error {
		return t.next.Delete(inner, ids)
	})
}

func (t *tracedImageService) List(
	ctx context.Context,
	req *request.ListImageReq,
) ([]resp.ImageItem, int64, error) {
	type listResult struct {
		List  []resp.ImageItem
		Total int64
	}

	out, err := runTraced(ctx, "image", "List", func(inner context.Context) (*listResult, error) {
		list, total, callErr := t.next.List(inner, req)
		return &listResult{List: list, Total: total}, callErr
	})
	if err != nil {
		return nil, 0, err
	}
	if out == nil {
		return nil, 0, nil
	}
	return out.List, out.Total, nil
}

func (t *tracedImageService) CleanOrphanFiles(ctx context.Context) error {
	return runTracedErr(ctx, "image", "CleanOrphanFiles", t.next.CleanOrphanFiles)
}

var _ contract.ImageServiceContract = (*tracedImageService)(nil)

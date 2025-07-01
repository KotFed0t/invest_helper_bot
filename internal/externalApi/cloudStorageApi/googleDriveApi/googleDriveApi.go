package googleDriveApi

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"path/filepath"
	"time"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/KotFed0t/invest_helper_bot/utils"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const downloadLinkTemplate = "https://drive.google.com/file/d/%s/view"

type GoogleDriveApi struct {
	srv *drive.Service
	cfg *config.Config
}

func New(ctx context.Context, cfg *config.Config) *GoogleDriveApi {
	srv, err := drive.NewService(ctx, option.WithCredentialsFile(cfg.GoogleDrive.CredentialsFile))
	if err != nil {
		slog.Error("failed on drive.NewService")
		panic(err)
	}
	return &GoogleDriveApi{srv: srv, cfg: cfg}
}

func (a *GoogleDriveApi) UploadFile(ctx context.Context, reader io.Reader, filename string) (downloadLink string, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "GoogleDriveApi.UploadFile"

	slog.Debug("UploadFile start", slog.String("rqID", rqID), slog.String("op", op), slog.String("filename", filename))

	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	slog.Debug("mime Type", slog.String("mime", mimeType))

	fileMeta := &drive.File{
		Name:     filename,
		MimeType: mimeType,
	}

	uploadedFile, err := a.srv.Files.
		Create(fileMeta).
		Media(reader). // автоматически разбивает на чанки по 16МБ и в случае ошибок сети ретраит их по 32сек максимум.(кастомный дедлайн можно настроить через MediaOptions)
		Context(ctx).
		Do()
	if err != nil {
		slog.Error("failed on uploading file to google drive", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return "", err
	}

	perm := &drive.Permission{
		Type: "anyone",
		Role: "reader",
	}

	_, err = a.srv.Permissions.Create(uploadedFile.Id, perm).Do()
	if err != nil {
		slog.Error("failed on creating permission to uploaded file in google drive", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return "", err
	}

	slog.Debug("UploadFile completed", slog.String("rqID", rqID), slog.String("op", op), slog.Any("uploadedFile", uploadedFile))

	return fmt.Sprintf(downloadLinkTemplate, uploadedFile.Id), nil
}

func (a *GoogleDriveApi) DeleteOldFiles(ctx context.Context) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "GoogleDriveApi.DeleteOldFiles"

	slog.Debug("DeleteOldFiles start", slog.String("rqID", rqID), slog.String("op", op))
	r, err := a.srv.Files.List().Fields("files(id, createdTime)").Do()
	if err != nil {
		slog.Error("failed on getting files", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return err
	}

	totalFiles := len(r.Files)
	deletedFiles := 0
	for _, f := range r.Files {
		createdTime, err := time.Parse(time.RFC3339, f.CreatedTime)
		if err != nil {
			slog.Error(
				"failed parse time",
				slog.String("rqID", rqID),
				slog.String("op", op),
				slog.String("err", err.Error()),
				slog.String("fileID", f.Id),
				slog.String("createdTime", f.CreatedTime),
			)
			continue
		}

		if createdTime.Before(time.Now().Add(-1 * a.cfg.GoogleDrive.FileTTL)) {
			err = a.srv.Files.Delete(f.Id).Do()
			if err != nil {
				slog.Error(
					"failed delete file",
					slog.String("rqID", rqID),
					slog.String("op", op),
					slog.String("err", err.Error()),
					slog.String("fileID", f.Id),
				)
			}
			deletedFiles++
		}
	}

	err = a.srv.Files.EmptyTrash().Do()
	if err != nil {
		slog.Error("failed empty trash", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
	}

	slog.Info("delete old files done", slog.Int("deletedFiles", deletedFiles), slog.Int("remaining files", totalFiles-deletedFiles))

	return nil
}

package mopan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/avast/retry-go"
	"github.com/foxxorcat/mopan-sdk-go"
)

type MoPan struct {
	model.Storage
	Addition
	client *mopan.MoClient

	userID string
}

func (d *MoPan) Config() driver.Config {
	return config
}

func (d *MoPan) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *MoPan) Init(ctx context.Context) error {
	login := func() error {
		data, err := d.client.Login(d.Phone, d.Password)
		if err != nil {
			return err
		}
		d.client.SetAuthorization(data.Token)

		info, err := d.client.GetUserInfo()
		if err != nil {
			return err
		}
		d.userID = info.UserID
		return nil
	}
	d.client = mopan.NewMoClient().
		SetRestyClient(base.RestyClient).
		SetOnAuthorizationExpired(func(_ error) error {
			err := login()
			if err != nil {
				d.Status = err.Error()
				op.MustSaveDriverStorage(d)
			}
			return err
		}).SetDeviceInfo(d.DeviceInfo)
	d.DeviceInfo = d.client.GetDeviceInfo()
	return login()
}

func (d *MoPan) Drop(ctx context.Context) error {
	d.client = nil
	d.userID = ""
	return nil
}

func (d *MoPan) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	var files []model.Obj
	for page := 1; ; page++ {
		data, err := d.client.QueryFiles(dir.GetID(), page, mopan.WarpParamOption(
			func(j mopan.Json) {
				j["orderBy"] = d.OrderBy
				j["descending"] = d.OrderDirection == "desc"
			},
			mopan.ParamOptionShareFile(d.CloudID),
		))
		if err != nil {
			return nil, err
		}

		if len(data.FileListAO.FileList)+len(data.FileListAO.FolderList) == 0 {
			break
		}

		files = append(files, utils.MustSliceConvert(data.FileListAO.FolderList, folderToObj)...)
		files = append(files, utils.MustSliceConvert(data.FileListAO.FileList, fileToObj)...)
	}
	return files, nil
}

func (d *MoPan) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	data, err := d.client.GetFileDownloadUrl(file.GetID(), mopan.WarpParamOption(mopan.ParamOptionShareFile(d.CloudID)))
	if err != nil {
		return nil, err
	}

	return &model.Link{
		URL: data.DownloadUrl,
	}, nil
}

func (d *MoPan) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
	f, err := d.client.CreateFolder(dirName, parentDir.GetID(), mopan.WarpParamOption(
		mopan.ParamOptionShareFile(d.CloudID),
	))
	if err != nil {
		return nil, err
	}
	return folderToObj(*f), nil
}

func (d *MoPan) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	return d.newTask(srcObj, dstDir, mopan.TASK_MOVE)
}

func (d *MoPan) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
	if srcObj.IsDir() {
		_, err := d.client.RenameFolder(srcObj.GetID(), newName, mopan.WarpParamOption(
			mopan.ParamOptionShareFile(d.CloudID),
		))
		if err != nil {
			return nil, err
		}
	} else {
		_, err := d.client.RenameFile(srcObj.GetID(), newName, mopan.WarpParamOption(
			mopan.ParamOptionShareFile(d.CloudID),
		))
		if err != nil {
			return nil, err
		}
	}
	return CloneObj(srcObj, srcObj.GetID(), newName), nil
}

func (d *MoPan) Copy(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	return d.newTask(srcObj, dstDir, mopan.TASK_COPY)
}

func (d *MoPan) newTask(srcObj, dstDir model.Obj, taskType mopan.TaskType) (model.Obj, error) {
	param := mopan.TaskParam{
		UserOrCloudID:       d.userID,
		Source:              1,
		TaskType:            taskType,
		TargetSource:        1,
		TargetUserOrCloudID: d.userID,
		TargetType:          1,
		TargetFolderID:      dstDir.GetID(),
		TaskStatusDetailDTOList: []mopan.TaskFileParam{
			{
				FileID:   srcObj.GetID(),
				IsFolder: srcObj.IsDir(),
				FileName: srcObj.GetName(),
			},
		},
	}
	if d.CloudID != "" {
		param.UserOrCloudID = d.CloudID
		param.Source = 2
		param.TargetSource = 2
		param.TargetUserOrCloudID = d.CloudID
	}

	task, err := d.client.AddBatchTask(param)
	if err != nil {
		return nil, err
	}

	for count := 0; count < 5; count++ {
		stat, err := d.client.CheckBatchTask(mopan.TaskCheckParam{
			TaskId:              task.TaskIDList[0],
			TaskType:            task.TaskType,
			TargetType:          1,
			TargetFolderID:      task.TargetFolderID,
			TargetSource:        param.TargetSource,
			TargetUserOrCloudID: param.TargetUserOrCloudID,
		})
		if err != nil {
			return nil, err
		}

		switch stat.TaskStatus {
		case 2:
			if err := d.client.CancelBatchTask(stat.TaskID, task.TaskType); err != nil {
				return nil, err
			}
			return nil, errors.New("file name conflict")
		case 4:
			if task.TaskType == mopan.TASK_MOVE {
				return CloneObj(srcObj, srcObj.GetID(), srcObj.GetName()), nil
			}
			return CloneObj(srcObj, stat.SuccessedFileIDList[0], srcObj.GetName()), nil
		}
		time.Sleep(time.Second)
	}
	return nil, nil
}

func (d *MoPan) Remove(ctx context.Context, obj model.Obj) error {
	_, err := d.client.DeleteToRecycle([]mopan.TaskFileParam{
		{
			FileID:   obj.GetID(),
			IsFolder: obj.IsDir(),
			FileName: obj.GetName(),
		},
	}, mopan.WarpParamOption(mopan.ParamOptionShareFile(d.CloudID)))
	return err
}

func (d *MoPan) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	file, err := utils.CreateTempFile(stream)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()

	initUpdload, err := d.client.InitMultiUpload(ctx, mopan.UpdloadFileParam{
		ParentFolderId: dstDir.GetID(),
		FileName:       stream.GetName(),
		FileSize:       stream.GetSize(),
		File:           file,
	}, mopan.WarpParamOption(
		mopan.ParamOptionShareFile(d.CloudID),
	))
	if err != nil {
		return nil, err
	}

	if !initUpdload.FileDataExists {
		parts, err := d.client.GetAllMultiUploadUrls(initUpdload.UploadFileID, initUpdload.PartInfo)
		if err != nil {
			return nil, err
		}
		d.client.CloudDiskStartBusiness()
		for i, part := range parts {
			if utils.IsCanceled(ctx) {
				return nil, ctx.Err()
			}

			err := retry.Do(func() error {
				if _, err := file.Seek(int64(part.PartNumber-1)*int64(initUpdload.PartSize), io.SeekStart); err != nil {
					return retry.Unrecoverable(err)
				}

				req, err := part.NewRequest(ctx, io.LimitReader(file, int64(initUpdload.PartSize)))
				if err != nil {
					return err
				}

				resp, err := base.HttpClient.Do(req)
				if err != nil {
					return err
				}

				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("upload err,code=%d", resp.StatusCode)
				}
				return nil
			},
				retry.Context(ctx),
				retry.Attempts(3),
				retry.Delay(time.Second),
				retry.MaxDelay(5*time.Second))
			if err != nil {
				return nil, err
			}
			up(100 * (i + 1) / len(parts))
		}
	}
	uFile, err := d.client.CommitMultiUploadFile(initUpdload.UploadFileID, nil)
	if err != nil {
		return nil, err
	}
	return &model.Object{
		ID:       uFile.UserFileID,
		Name:     uFile.FileName,
		Size:     int64(uFile.FileSize),
		Modified: time.Time(uFile.CreateDate),
	}, nil
}

var _ driver.Driver = (*MoPan)(nil)
var _ driver.MkdirResult = (*MoPan)(nil)
var _ driver.MoveResult = (*MoPan)(nil)
var _ driver.RenameResult = (*MoPan)(nil)
var _ driver.Remove = (*MoPan)(nil)
var _ driver.CopyResult = (*MoPan)(nil)
var _ driver.PutResult = (*MoPan)(nil)

package aliyundrive_share2_open

import (
	"context"
	"errors"
	"fmt"
	"github.com/alist-org/alist/v3/internal/conf"
	"log"
	"net/http"
	"time"

	"github.com/Xhofe/rateg"
	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/cron"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/go-resty/resty/v2"
)

var ParentFileId = ""
var DriveId = ""
var lastTime int64 = 0

type AliyundriveShare2Open struct {
	base string
	model.Storage
	Addition
	AccessToken     string
	AccessTokenOpen string
	ShareToken      string
	DriveId         string
	cron            *cron.Cron

	limitList func(ctx context.Context, dir model.Obj) ([]model.Obj, error)
	limitLink func(ctx context.Context, file model.Obj) (*model.Link, error)
}

func (d *AliyundriveShare2Open) Config() driver.Config {
	return config
}

func (d *AliyundriveShare2Open) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *AliyundriveShare2Open) Init(ctx context.Context) error {
	if lastTime > 0 {
		diff := lastTime + 1000 - time.Now().UnixMilli()
		time.Sleep(time.Duration(diff) * time.Millisecond)
	}

	err := d.refreshToken(false)
	if err != nil {
		return err
	}
	err = d.getShareToken()
	if err != nil {
		return err
	}

	if d.OauthTokenURL == "" {
		d.OauthTokenURL = conf.Conf.OpenTokenAuthUrl
	}

	err = d.refreshOpenToken(false)
	if err != nil {
		return err
	}

	d.getDriveId()
	d.createFolderOpen()

	d.limitList = rateg.LimitFnCtx(d.list, rateg.LimitFnOption{
		Limit:  4,
		Bucket: 1,
	})
	d.limitLink = rateg.LimitFnCtx(d.link, rateg.LimitFnOption{
		Limit:  1,
		Bucket: 1,
	})
	return nil
}

func (d *AliyundriveShare2Open) Drop(ctx context.Context) error {
	if d.cron != nil {
		d.cron.Stop()
	}
	d.DriveId = ""
	return nil
}

func (d *AliyundriveShare2Open) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	if d.limitList == nil {
		return nil, fmt.Errorf("driver not init")
	}
	return d.limitList(ctx, dir)
}

func (d *AliyundriveShare2Open) list(ctx context.Context, dir model.Obj) ([]model.Obj, error) {
	files, err := d.getFiles(dir.GetID())
	if err != nil {
		return nil, err
	}
	return utils.SliceConvert(files, func(src File) (model.Obj, error) {
		return fileToObj(src), nil
	})
}

func (d *AliyundriveShare2Open) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	if d.limitLink == nil {
		return nil, fmt.Errorf("driver not init")
	}
	return d.limitLink(ctx, file)
}

func (d *AliyundriveShare2Open) link(ctx context.Context, file model.Obj) (*model.Link, error) {
	// 1. 转存资源
	// 2. 获取链接
	// 3. 删除文件
	log.Printf("获取文件直链 %v %v %v %v", d.DriveId, file.GetName(), file.GetID(), file.GetSize())
	fileId, err := d.saveFile(file.GetID())
	if err != nil {
		return nil, err
	}

	go d.deleteDelay(fileId)

	newFile := MyFile{
		FileId: fileId,
		Name:   "livp",
	}

	return d.getOpenLink(newFile)
}

func (d *AliyundriveShare2Open) saveFile(fileId string) (string, error) {
	data := base.Json{
		"requests": []base.Json{
			{
				"body": base.Json{
					"file_id":           fileId,
					"share_id":          d.ShareId,
					"auto_rename":       true,
					"to_parent_file_id": ParentFileId,
					"to_drive_id":       d.DriveId,
				},
				"headers": base.Json{
					"Content-Type": "application/json",
				},
				"id":     "0",
				"method": "POST",
				"url":    "/file/copy",
			},
		},
		"resource": "file",
	}

	// TODO: delete this
	err := d.getShareToken()
	if err != nil {
		log.Printf("getShareToken failed: %v", err)
		return "", err
	}

	res, err := d.request("https://api.aliyundrive.com/adrive/v2/batch", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data)
	})
	if err != nil {
		log.Printf("saveFile failed: %v", err)
		return "", err
	}
	newFile := utils.Json.Get(res, "responses", 0, "body", "file_id").ToString()
	return newFile, nil
}

func (d *AliyundriveShare2Open) getOpenLink(file model.Obj) (*model.Link, error) {
	res, err := d.requestOpen("/adrive/v1.0/openFile/getDownloadUrl", http.MethodPost, func(req *resty.Request) {
		req.SetBody(base.Json{
			"drive_id":   d.DriveId,
			"file_id":    file.GetID(),
			"expire_sec": 14400,
		})
	})
	if err != nil {
		log.Printf("getOpenLink failed: %v", err)
		return nil, err
	}
	url := utils.Json.Get(res, "url").ToString()
	if url == "" {
		if utils.Ext(file.GetName()) != "livp" {
			return nil, errors.New("get download url failed: " + string(res))
		}
		url = utils.Json.Get(res, "streamsUrl", "mov").ToString()
	}

	exp := time.Hour
	return &model.Link{
		URL:        url,
		Expiration: &exp,
	}, nil
}

func (d *AliyundriveShare2Open) deleteDelay(fileId string) error {
	time.Sleep(1200 * time.Millisecond)
	return d.deleteOpen(fileId)
}

func (d *AliyundriveShare2Open) deleteOpen(fileId string) error {
	_, err := d.requestOpen("/adrive/v1.0/openFile/delete", http.MethodPost, func(req *resty.Request) {
		req.SetBody(base.Json{
			"drive_id": d.DriveId,
			"file_id":  fileId,
		})
	})
	if err != nil {
		log.Printf("删除文件%v失败： %v", fileId, err)
	}
	return err
}

func (d *AliyundriveShare2Open) delete(fileId string) error {
	data := base.Json{
		"requests": []base.Json{
			{
				"body": base.Json{
					"drive_id": d.DriveId,
					"file_id":  fileId,
				},
				"headers": base.Json{
					"Content-Type": "application/json",
				},
				"id":     fileId,
				"method": "POST",
				"url":    "/file/delete",
			},
		},
		"resource": "file",
	}

	_, err := d.request("https://api.aliyundrive.com/v3/batch", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data)
	})
	if err != nil {
		log.Printf("删除文件%v失败： %v", fileId, err)
	}

	return err
}

func (d *AliyundriveShare2Open) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
	if args.Method != "video_preview" {
		return nil, errs.NotSupport
	}

	log.Printf("获取文件链接 %v %v %v %v", d.DriveId, args.Obj.GetName(), args.Obj.GetID(), args.Obj.GetSize())
	fileId, err := d.saveFile(args.Obj.GetID())
	if err != nil {
		return nil, err
	}

	go d.deleteDelay(fileId)

	var resp base.Json
	var uri string
	data := base.Json{
		"drive_id": d.DriveId,
		"file_id":  fileId,
	}
	switch args.Method {
	case "video_preview":
		uri = "/adrive/v1.0/openFile/getVideoPreviewPlayInfo"
		data["category"] = "live_transcoding"
		data["url_expire_sec"] = 14400
	default:
		return nil, errs.NotSupport
	}
	_, err = d.requestOpen(uri, http.MethodPost, func(req *resty.Request) {
		req.SetBody(data).SetResult(&resp)
	})

	if err != nil {
		log.Printf("获取文件链接失败：%v", err)
		return nil, err
	}
	return resp, nil
}

var _ driver.Driver = (*AliyundriveShare2Open)(nil)

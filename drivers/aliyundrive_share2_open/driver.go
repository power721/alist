package aliyundrive_share2_open

import (
	"context"
	"errors"
	"fmt"
	"github.com/alist-org/alist/v3/internal/conf"
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
	log "github.com/sirupsen/logrus"
)

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
	err := d.refreshToken(false)
	if err != nil {
		return err
	}
	err = d.getShareToken()
	if err != nil {
		return err
	}
	d.cron = cron.NewCron(time.Hour * 2)
	d.cron.Do(func() {
		err := d.refreshToken(true)
		if err != nil {
			log.Errorf("%+v", err)
		}
	})

	if d.OauthTokenURL == "" {
		d.OauthTokenURL = conf.Conf.OpenTokenAuthUrl
	}

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
	data := base.Json{
		"requests": []base.Json{
			{
				"body": base.Json{
					"file_id":           file.GetID(),
					"share_id":          d.ShareId,
					"auto_rename":       true,
					"to_parent_file_id": "root",
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

	err := d.getShareToken()
	if err != nil {
		return nil, err
	}

	utils.Log.Printf("转存资源 %v %v", file.GetID(), file.GetName())
	res, err := d.request("https://api.aliyundrive.com/adrive/v2/batch", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data)
	})
	if err != nil {
		return nil, err
	}
	fileId := utils.Json.Get(res, "responses", 0, "body", "file_id").ToString()
	newFile := MyFile{
		FileId: fileId,
		Name:   "livp",
	}
	return d.getDownloadUrl(ctx, newFile)
}

func (d *AliyundriveShare2Open) getDownloadUrl(ctx context.Context, file model.Obj) (*model.Link, error) {
    utils.Log.Printf("获取文件直链 %v %v", d.DriveId, file.GetID())
	data := base.Json{
		"drive_id":   d.DriveId,
		"file_id":    file.GetID(),
		"expire_sec": 14400,
	}
	res, err := d.request("https://api.aliyundrive.com/v2/file/get_download_url", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data)
	})
	if err != nil {
		return nil, err
	}
	return &model.Link{
		Header: http.Header{
			"Referer": []string{"https://www.aliyundrive.com/"},
		},
		URL: utils.Json.Get(res, "url").ToString(),
	}, nil
}

func (d *AliyundriveShare2Open) getOpenLink(ctx context.Context, file model.Obj) (*model.Link, error) {
	utils.Log.Printf("获取文件直链 %v %v", d.DriveId, file.GetID())
	res, err := d.requestOpen("/adrive/v1.0/openFile/getDownloadUrl", http.MethodPost, func(req *resty.Request) {
		req.SetBody(base.Json{
			"drive_id":   d.DriveId,
			"file_id":    file.GetID(),
			"expire_sec": 14400,
		})
	})
	if err != nil {
		return nil, err
	}
	url := utils.Json.Get(res, "url").ToString()
	if url == "" {
		if utils.Ext(file.GetName()) != "livp" {
			return nil, errors.New("get download url failed: " + string(res))
		}
		url = utils.Json.Get(res, "streamsUrl", "mov").ToString()
	}

	go d.deleteDelay(file)

	exp := time.Hour
	return &model.Link{
		URL:        url,
		Expiration: &exp,
	}, nil
}

func (d *AliyundriveShare2Open) deleteDelay(file model.Obj) error {
	time.Sleep(1 * time.Second)
	return d.delete(file)
}

func (d *AliyundriveShare2Open) delete(file model.Obj) error {
	data := base.Json{
		"requests": []base.Json{
			{
				"body": base.Json{
					"id":       file.GetID(),
					"file_id":  file.GetID(),
					"drive_id": d.DriveId,
				},
				"headers": base.Json{
					"Content-Type": "application/json",
				},
				"id":     "0",
				"method": "POST",
				"url":    "/file/delete",
			},
		},
		"resource": "file",
	}

	_, err := d.request("https://api.aliyundrive.com/v3/batch", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data)
	})

	return err
}

func (d *AliyundriveShare2Open) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var resp base.Json
	var url string
	data := base.Json{
		"share_id": d.ShareId,
		"file_id":  args.Obj.GetID(),
	}
	switch args.Method {
	case "doc_preview":
		url = "https://api.aliyundrive.com/v2/file/get_office_preview_url"
	case "video_preview":
		url = "https://api.aliyundrive.com/v2/file/get_video_preview_play_info"
		data["category"] = "live_transcoding"
	default:
		return nil, errs.NotSupport
	}
	_, err := d.request(url, http.MethodPost, func(req *resty.Request) {
		req.SetBody(data).SetResult(&resp)
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

var _ driver.Driver = (*AliyundriveShare2Open)(nil)

package aliyundrive_share2_open

import (
	"context"
	"fmt"
	"github.com/Xhofe/rateg"
	_115 "github.com/alist-org/alist/v3/drivers/115"
	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/internal/setting"
	"github.com/alist-org/alist/v3/pkg/cron"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
	"time"
)

type AliyundriveShare2Open struct {
	base string
	model.Storage
	Addition
	cron *cron.Cron

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
	if !initialized {
		err := d.refreshToken(false)
		if err != nil {
			log.Errorf("refreshToken error: %v", err)
			return err
		}

		d.getUser()

		lazyLoad = setting.GetBool("ali_lazy_load")
		ClientID = setting.GetStr("open_api_client_id")
		ClientSecret = setting.GetStr("open_api_client_secret")
		log.Printf("Open API Client ID: %v", ClientID)

		err = d.refreshOpenToken(false)
		if err != nil {
			log.Errorf("refreshOpenToken error: %v", err)
			return err
		}

		d.getDriveId()
		d.createFolderOpen()
		d.clean()

		initialized = true
	}

	if !lazyLoad {
		if lastTime > 0 {
			diff := lastTime + DelayTime - time.Now().UnixMilli()
			time.Sleep(time.Duration(diff) * time.Millisecond)
		}

		err := d.getShareToken()
		if err != nil {
			log.Errorf("getShareToken error: %v", err)
			return err
		}
	}

	d.limitList = rateg.LimitFnCtx(d.list, rateg.LimitFnOption{
		Limit:  4,
		Bucket: 1,
	})
	return nil
}

func (d *AliyundriveShare2Open) Drop(ctx context.Context) error {
	if d.cron != nil {
		d.cron.Stop()
	}
	DriveId = ""
	return nil
}

func (d *AliyundriveShare2Open) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	if d.limitList == nil {
		return nil, fmt.Errorf("driver not init")
	}
	return d.limitList(ctx, dir)
}

func (d *AliyundriveShare2Open) list(ctx context.Context, dir model.Obj) ([]model.Obj, error) {
	if d.ShareToken == "" {
		err := d.getShareToken()
		if err != nil {
			log.Warnf("getShareToken error: %v", err)
			return nil, err
		}
	}

	files, err := d.getFiles(dir.GetID())
	if err != nil {
		log.Warnf("list files error: %v", err)
		return nil, err
	}
	return utils.SliceConvert(files, func(src File) (model.Obj, error) {
		return fileToObj(src), nil
	})
}

func (d *AliyundriveShare2Open) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	// 1. 转存资源
	// 2. 获取链接
	// 3. 删除文件
	log.Infof("获取文件直链 %v %v %v %v", DriveId, file.GetName(), file.GetID(), file.GetSize())
	fileId, err := d.saveFile(file.GetID())
	if err != nil {
		return nil, err
	}

	newFile := MyFile{
		FileId: fileId,
		Name:   "livp",
	}

	link, hash, err := d.getOpenLink(newFile)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(file.GetName(), ".md") || !setting.GetBool(conf.AliTo115) {
		return link, err
	}

	driver115 := op.Get115Driver()
	if driver115 != nil {
		myFile := MyFile{
			FileId:   fileId,
			Name:     file.GetName(),
			Size:     file.GetSize(),
			HashInfo: utils.NewHashInfo(utils.SHA1, hash),
		}
		link115, err2 := d.saveTo115(ctx, driver115.(*_115.Pan115), myFile, link, args)
		if err2 == nil {
			link = link115
		}
	}
	return link, err
}

func (d *AliyundriveShare2Open) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
	if args.Method == "share_info" {
		d.getShareToken()
		data := base.Json{
			"shareId":    d.ShareId,
			"sharePwd":   d.SharePwd,
			"shareToken": d.ShareToken,
			"fileId":     args.Obj.GetID(),
		}
		return data, nil
	}

	if args.Method != "video_preview" {
		return nil, errs.NotSupport
	}

	log.Infof("获取文件链接 %v %v %v %v", DriveId, args.Obj.GetName(), args.Obj.GetID(), args.Obj.GetSize())
	fileId, err := d.saveFile(args.Obj.GetID())
	if err != nil {
		return nil, err
	}

	var resp VideoPreviewResponse
	var uri string
	data := base.Json{
		"drive_id": DriveId,
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

	go d.deleteDelay(fileId)

	if err != nil {
		log.Errorf("获取文件链接失败：%v", err)
		return nil, err
	}

	if args.Data == "preview" {
		url, _, _ := d.getDownloadUrl(fileId)
		if url != "" {
			resp.PlayInfo.Videos = append(resp.PlayInfo.Videos, LiveTranscoding{
				TemplateId: "原画",
				Status:     "finished",
				Url:        url,
			})
		}
	}

	return resp, nil
}

var _ driver.Driver = (*AliyundriveShare2Open)(nil)

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
	d.limitList = rateg.LimitFnCtx(d.list, rateg.LimitFnOption{
		Limit:  4,
		Bucket: 1,
	})

	if conf.LazyLoad && !conf.StoragesLoaded {
		return nil
	}

	err := d.Validate()
	time.Sleep(1500 * time.Millisecond)
	return err
}

func (d *AliyundriveShare2Open) Drop(ctx context.Context) error {
	if d.cron != nil {
		d.cron.Stop()
	}
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
	ali, err := getAliOpenDriver(idx)
	if err != nil {
		return nil, err
	}
	log.Infof("[%v] 获取阿里云盘文件直链 %v %v %v %v", ali.ID, ali.DriveId, file.GetName(), file.GetID(), file.GetSize())
	fileId, err := d.saveFile(ali, file.GetID())
	idx++
	if err != nil {
		return nil, err
	}

	newFile := MyFile{
		FileId: fileId,
		Name:   "livp",
	}

	link, hash, err := d.getOpenLink(ali, newFile)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(file.GetName(), ".md") || !setting.GetBool(conf.AliTo115) {
		return link, err
	}

	driver115 := op.Get115Driver(idx2)
	if driver115 != nil {
		myFile := MyFile{
			FileId:   fileId,
			Name:     file.GetName(),
			Size:     file.GetSize(),
			HashInfo: utils.NewHashInfo(utils.SHA1, hash),
		}
		link115, err2 := d.saveTo115(ctx, driver115.(*_115.Pan115), myFile, link, args)
		idx2++
		if err2 == nil {
			link = link115
		}
	}
	return link, err
}

func (d *AliyundriveShare2Open) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
	ali, err := getAliOpenDriver(idx)
	if err != nil {
		return nil, err
	}

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

	log.Infof("[%v] 获取文件链接 %v %v %v %v", ali.ID, ali.DriveId, args.Obj.GetID(), args.Obj.GetName(), args.Obj.GetSize())
	fileId, err := d.saveFile(ali, args.Obj.GetID())
	idx++
	if err != nil {
		return nil, err
	}

	var resp VideoPreviewResponse
	var uri string
	data := base.Json{
		"drive_id": ali.DriveId,
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
	_, err = ali.Request(uri, http.MethodPost, func(req *resty.Request) {
		req.SetBody(data).SetResult(&resp)
	})

	go d.deleteDelay(ali, fileId)

	if err != nil {
		log.Errorf("获取文件链接失败：%v", err)
		return nil, err
	}

	if args.Data == "preview" {
		url, _, _ := d.getDownloadUrl(ali, fileId)
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

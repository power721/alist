package baidu_share

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/alist-org/alist/v3/drivers/baidu_netdisk"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/pkg/cookie"
	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
	"net/http"
	"path"
	"time"
)

var idx = 0

type BaiduShare2 struct {
	model.Storage
	Addition
	client *resty.Client

	Token string
}

func (d *BaiduShare2) Config() driver.Config {
	return config
}

func (d *BaiduShare2) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *BaiduShare2) Init(ctx context.Context) error {
	d.client = resty.New().
		SetBaseURL("https://pan.baidu.com").
		SetHeader("User-Agent", "netdisk").
		SetHeader("Referer", "https://pan.baidu.com")
	return d.Validate()
}

func (d *BaiduShare2) Drop(ctx context.Context) error {
	return nil
}

func (d *BaiduShare2) Validate() error {
	if d.SharePwd == "" {
		api := "/s/" + d.ShareId
		res, err := d.client.R().
			Get(api)
		if err != nil {
			log.Warnf("error: %v", err)
			return err
		}
		BDCLND := cookie.GetCookie(res.Cookies(), "BDCLND")
		if BDCLND != nil {
			d.Token = BDCLND.Value
		}
	} else {
		api := "/share/verify?channel=chunlei&clienttype=0&web=1&app_id=250528&surl=" + d.ShareId[1:]
		data := map[string]string{
			"pwd": d.SharePwd,
		}
		respJson := struct {
			Errno   int64  `json:"errno"`
			Message string `json:"err_msg"`
			Token   string `json:"randsk"`
		}{}
		_, err := d.client.R().
			SetFormData(data).
			SetHeader("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8").
			SetResult(&respJson).
			Post(api)
		if err != nil {
			log.Warnf("error: %v", err)
			return err
		}
		log.Debugf("respJson: %v", respJson)
		if respJson.Errno != 0 {
			log.Warnf("error: %v", respJson.Message)
			return errors.New(respJson.Message)
		}
		d.Token = respJson.Token
	}
	log.Debugf("Share Token: %v", d.Token)
	return nil
}

func (d *BaiduShare2) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	reqDir := dir.GetPath()
	isRoot := "0"
	if reqDir == d.RootFolderPath {
		reqDir = path.Join("/", reqDir)
	}
	if reqDir == "/" {
		isRoot = "1"
		reqDir = ""
	}
	objs := []model.Obj{}
	var err error
	var page = 1
	more := true
	for more && err == nil {
		respJson := struct {
			Errno int64 `json:"errno"`
			List  []struct {
				Fsid  json.Number `json:"fs_id"`
				Isdir json.Number `json:"isdir"`
				Path  string      `json:"path"`
				Name  string      `json:"server_filename"`
				Mtime json.Number `json:"server_mtime"`
				Size  json.Number `json:"size"`
			} `json:"list"`
		}{}
		query := map[string]string{
			"app_id":     "250528",
			"channel":    "chunlei",
			"clienttype": "0",
			"desc":       "0",
			"showempty":  "0",
			"web":        "1",
			"view_mode":  "1",
			"num":        "100",
			"order":      "name",
			"root":       isRoot,
			"dir":        reqDir,
			"shorturl":   d.ShareId[1:],
			"page":       fmt.Sprint(page),
		}
		res, e := d.client.R().
			SetCookie(&http.Cookie{Name: "BDCLND", Value: d.Token}).
			SetResult(&respJson).
			SetQueryParams(query).
			Get("/share/list")
		err = e
		log.Infof("%v result: %v", reqDir, res.String())
		more = false
		if err == nil {
			if res.IsSuccess() && respJson.Errno == 0 {
				page++
				for _, v := range respJson.List {
					size, _ := v.Size.Int64()
					mtime, _ := v.Mtime.Int64()
					objs = append(objs, &model.Object{
						ID:       v.Fsid.String(),
						Path:     v.Path,
						Name:     v.Name,
						Size:     size,
						Modified: time.Unix(mtime, 0),
						IsFolder: v.Isdir.String() == "1",
					})
				}
			} else {
				err = fmt.Errorf("%s", res.Body())
			}
		}
	}
	return objs, err
}

func (d *BaiduShare2) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	storage := op.GetFirstDriver("BaiduNetdisk", idx)
	if storage == nil {
		return nil, errors.New("找不到百度网盘帐号")
	}
	bd := storage.(*baidu_netdisk.BaiduNetdisk)
	Cookie := bd.Cookie + "; " + "BDCLND=" + d.Token

	link := model.Link{Header: d.client.Header}

	respJson := struct {
		Errno int64 `json:"errno"`
		List  []struct {
			Fsid  json.Number `json:"fs_id"`
			Isdir json.Number `json:"isdir"`
			Path  string      `json:"path"`
			Name  string      `json:"server_filename"`
			Mtime json.Number `json:"server_mtime"`
			Size  json.Number `json:"size"`
		} `json:"list"`
	}{}
	data := map[string]string{
		"fsidlist": fmt.Sprintf("[%v]", file.GetID()),
		"path":     "/" + conf.TempDirName,
	}
	query := map[string]string{
		"app_id":     "250528",
		"channel":    "chunlei",
		"clienttype": "0",
		"web":        "1",
		"async":      "1",
		"ondup":      "newcopy",
		"shareid":    "?",
		"from":       fmt.Sprint(bd.UK),
		"sekey":      d.Token,
	}
	res, err := d.client.R().
		SetFormData(data).
		SetHeader("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8").
		SetHeader("Cookie", Cookie).
		SetResult(&respJson).
		SetQueryParams(query).
		Get("/share/transfer")

	if res.IsSuccess() {

	}

	return &link, err
}

func (d *BaiduShare2) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) error {
	// TODO create folder, optional
	return errs.NotSupport
}

func (d *BaiduShare2) Move(ctx context.Context, srcObj, dstDir model.Obj) error {
	// TODO move obj, optional
	return errs.NotSupport
}

func (d *BaiduShare2) Rename(ctx context.Context, srcObj model.Obj, newName string) error {
	// TODO rename obj, optional
	return errs.NotSupport
}

func (d *BaiduShare2) Copy(ctx context.Context, srcObj, dstDir model.Obj) error {
	// TODO copy obj, optional
	return errs.NotSupport
}

func (d *BaiduShare2) Remove(ctx context.Context, obj model.Obj) error {
	// TODO remove obj, optional
	return errs.NotSupport
}

func (d *BaiduShare2) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) error {
	// TODO upload file, optional
	return errs.NotSupport
}

//func (d *Template) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
//	return nil, errs.NotSupport
//}

var _ driver.Driver = (*BaiduShare2)(nil)

package baidu_share

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/alist-org/alist/v3/drivers/baidu_netdisk"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/pkg/cookie"
	log "github.com/sirupsen/logrus"
	"io"
	"net/url"
	"path"
	"time"

	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/go-resty/resty/v2"
)

var idx = 0

type BaiduShare2 struct {
	model.Storage
	Addition
	client *resty.Client

	Token string

	info struct {
		Root    string
		Seckey  string
		Shareid string
		Uk      string
	}
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
	if d.SharePwd != "" {
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
			SetResult(&respJson).
			SetHeader("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8").
			Post(api)
		if err != nil {
			log.Warnf("error: %v", err)
			return err
		}
		log.Infof("respJson: %v", respJson)
		if respJson.Errno != 0 {
			log.Warnf("error: %v", respJson.Message)
			return errors.New(respJson.Message)
		}
		d.Token = respJson.Token
	} else {
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
	}
	log.Infof("Share Token: %v", d.Token)
	return nil
}

func (d *BaiduShare2) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	storage := op.GetFirstDriver("BaiduNetdisk", idx)
	if storage == nil {
		return nil, errors.New("找不到百度网盘帐号")
	}
	bd := storage.(*baidu_netdisk.BaiduNetdisk)
	Cookie := bd.Cookie + "; " + "BDCLND=" + d.Token

	// TODO return the files list, required
	reqDir := dir.GetPath()
	isRoot := "0"
	if reqDir == d.RootFolderPath {
		reqDir = path.Join(d.info.Root, reqDir)
	}
	if reqDir == d.info.Root {
		isRoot = "1"
	}
	objs := []model.Obj{}
	var err error
	var page uint64 = 1
	more := true
	for more && err == nil {
		respJson := struct {
			Errno int64 `json:"errno"`
			Data  struct {
				More bool `json:"has_more"`
				List []struct {
					Fsid  json.Number `json:"fs_id"`
					Isdir json.Number `json:"isdir"`
					Path  string      `json:"path"`
					Name  string      `json:"server_filename"`
					Mtime json.Number `json:"server_mtime"`
					Size  json.Number `json:"size"`
				} `json:"list"`
			} `json:"data"`
		}{}
		resp, e := d.client.R().
			SetBody(url.Values{
				"dir":      {reqDir},
				"num":      {"1000"},
				"order":    {"time"},
				"page":     {fmt.Sprint(page)},
				"pwd":      {d.SharePwd},
				"root":     {isRoot},
				"shorturl": {d.ShareId},
			}.Encode()).
			SetHeader("Cookie", Cookie).
			SetResult(&respJson).
			Post("share/wxlist?channel=weixin&version=2.2.2&clienttype=25&web=1")
		err = e
		if err == nil {
			if resp.IsSuccess() && respJson.Errno == 0 {
				page++
				more = respJson.Data.More
				for _, v := range respJson.Data.List {
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
				err = fmt.Errorf(" %s; %s; ", resp.Status(), resp.Body())
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

	// TODO return link of file, required
	link := model.Link{Header: d.client.Header}
	sign := ""
	stamp := ""
	signJson := struct {
		Errno int64 `json:"errno"`
		Data  struct {
			Stamp json.Number `json:"timestamp"`
			Sign  string      `json:"sign"`
		} `json:"data"`
	}{}
	resp, err := d.client.R().
		SetQueryParam("surl", d.ShareId).
		SetResult(&signJson).
		Get("share/tplconfig?fields=sign,timestamp&channel=chunlei&web=1&app_id=250528&clienttype=0")
	if err == nil {
		if resp.IsSuccess() && signJson.Errno == 0 {
			stamp = signJson.Data.Stamp.String()
			sign = signJson.Data.Sign
		} else {
			err = fmt.Errorf(" %s; %s; ", resp.Status(), resp.Body())
		}
	}
	if err == nil {
		respJson := struct {
			Errno int64 `json:"errno"`
			List  [1]struct {
				Dlink string `json:"dlink"`
			} `json:"list"`
		}{}
		resp, err = d.client.R().
			SetQueryParam("sign", sign).
			SetQueryParam("timestamp", stamp).
			SetBody(url.Values{
				"encrypt":   {"0"},
				"extra":     {fmt.Sprintf(`{"sekey":"%s"}`, d.info.Seckey)},
				"fid_list":  {fmt.Sprintf("[%s]", file.GetID())},
				"primaryid": {d.info.Shareid},
				"product":   {"share"},
				"type":      {"nolimit"},
				"uk":        {d.info.Uk},
			}.Encode()).
			SetHeader("Cookie", Cookie).
			SetResult(&respJson).
			Post("api/sharedownload?app_id=250528&channel=chunlei&clienttype=12&web=1")
		if err == nil {
			if resp.IsSuccess() && respJson.Errno == 0 && respJson.List[0].Dlink != "" {
				link.URL = respJson.List[0].Dlink
			} else {
				err = fmt.Errorf(" %s; %s; ", resp.Status(), resp.Body())
			}
		}
		if err == nil {
			resp, err = d.client.R().
				SetDoNotParseResponse(true).
				Get(link.URL)
			if err == nil {
				defer resp.RawBody().Close()
				if resp.IsError() {
					byt, _ := io.ReadAll(resp.RawBody())
					err = fmt.Errorf(" %s; %s; ", resp.Status(), byt)
				}
			}
		}
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

package aliyundrive_share2_open

import (
	"context"
	"errors"
	"github.com/SheltonZhu/115driver/pkg/driver"
	_115 "github.com/alist-org/alist/v3/drivers/115"
	"github.com/alist-org/alist/v3/drivers/aliyundrive_open"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/setting"
	"github.com/alist-org/alist/v3/internal/stream"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/go-resty/resty/v2"
	"net/http"
	"strings"
	"time"

	"github.com/alist-org/alist/v3/drivers/base"
	log "github.com/sirupsen/logrus"
)

const (
	// CanaryHeaderKey CanaryHeaderValue for lifting rate limit restrictions
	CanaryHeaderKey   = "X-Canary"
	CanaryHeaderValue = "client=web,app=share,version=v2.3.1"
)

var idx = 0
var lastId = ""

var idx2 = 0
var lastId2 = ""

func (d *AliyundriveShare2Open) getShareToken() error {
	data := base.Json{
		"share_id": d.ShareId,
	}
	if d.SharePwd != "" {
		data["share_pwd"] = d.SharePwd
	}
	var e ErrorResp
	var resp ShareTokenResp
	_, err := base.RestyClient.R().
		SetResult(&resp).SetError(&e).SetBody(data).
		Post("https://api.alipan.com/v2/share_link/get_share_token")
	if err != nil {
		return err
	}
	if e.Code != "" {
		return errors.New(e.Message)
	}
	d.ShareToken = resp.ShareToken
	log.Debug("getShareToken", d.ShareId, d.ShareToken)
	return nil
}

func (d *AliyundriveShare2Open) saveFile(ali *aliyundrive_open.AliyundriveOpen, fileId string) (string, error) {
	data := base.Json{
		"requests": []base.Json{
			{
				"body": base.Json{
					"file_id":           fileId,
					"share_id":          d.ShareId,
					"auto_rename":       true,
					"to_parent_file_id": ali.TempDirId,
					"to_drive_id":       ali.DriveId,
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

	res, err := d.request(ali, "https://api.alipan.com/adrive/v4/batch", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data)
	})

	if err != nil {
		log.Errorf("保存文件失败: %v", err)
		if strings.Contains(err.Error(), "share_id doesn't match. share_token") {
			log.Infof("getShareToken: %v", d.ShareId)
			d.getShareToken()
		}
		return "", err
	}

	msg := utils.Json.Get(res, "responses", 0, "body", "message").ToString()
	if msg != "" {
		log.Infof("请求结果 : %v", string(res))
		log.Errorf("保存文件失败 : %v", msg)
		if strings.Contains(msg, "share_id doesn't match. share_token") {
			log.Infof("getShareToken: %v", d.ShareId)
			d.getShareToken()
		}
		return "", errors.New(msg)
	}

	newFile := utils.Json.Get(res, "responses", 0, "body", "file_id").ToString()
	return newFile, nil
}

func (d *AliyundriveShare2Open) getOpenLink(ali *aliyundrive_open.AliyundriveOpen, file model.Obj) (*model.Link, string, error) {
	res, err := ali.Request("/adrive/v1.0/openFile/getDownloadUrl", http.MethodPost, func(req *resty.Request) {
		req.SetBody(base.Json{
			"drive_id":   ali.DriveId,
			"file_id":    file.GetID(),
			"expire_sec": 14400,
		})
	})

	go d.deleteDelay(ali, file.GetID())

	if err != nil {
		log.Errorf("getOpenLink failed: %v", err)
		return nil, "", err
	}
	url := utils.Json.Get(res, "url").ToString()
	if url == "" {
		if utils.Ext(file.GetName()) != "livp" {
			return nil, "", errors.New("get download url failed: " + string(res))
		}
		url = utils.Json.Get(res, "streamsUrl", "mov").ToString()
	}
	hash := utils.Json.Get(res, "content_hash").ToString()

	exp := 895 * time.Second
	return &model.Link{
		URL:        url,
		Expiration: &exp,
		Header: http.Header{
			"Referer":    []string{"https://www.alipan.com/"},
			"User-Agent": []string{conf.UserAgent},
		},
		Concurrency: conf.AliThreads,
		PartSize:    conf.AliChunkSize * utils.KB,
	}, hash, nil
}

func (d *AliyundriveShare2Open) getDownloadUrl(ali *aliyundrive_open.AliyundriveOpen, fileId string) (string, string, error) {
	log.Infof("getDownloadUrl %v %v", ali.DriveId, fileId)
	res, err := ali.Request("/adrive/v1.0/openFile/getDownloadUrl", http.MethodPost, func(req *resty.Request) {
		req.SetBody(base.Json{
			"drive_id":   ali.DriveId,
			"file_id":    fileId,
			"expire_sec": 14400,
		})
	})

	if err != nil {
		log.Errorf("getDownloadUrl failed: %v", err)
		return "", "", err
	}
	url := utils.Json.Get(res, "url").ToString()
	if url == "" {
		url = utils.Json.Get(res, "streamsUrl", "mov").ToString()
	}

	hash := utils.Json.Get(res, "content_hash").ToString()

	return url, hash, nil
}

func (d *AliyundriveShare2Open) deleteDelay(ali *aliyundrive_open.AliyundriveOpen, fileId string) {
	delayTime := setting.GetInt(conf.DeleteDelayTime, 900)
	if delayTime == 0 {
		return
	}

	log.Infof("[%v] Delete aliyun temp file %v after %v seconds.", ali.ID, fileId, delayTime)
	time.Sleep(time.Duration(delayTime) * time.Second)
	d.deleteOpen(ali, fileId)
}

func (d *AliyundriveShare2Open) deleteOpen(ali *aliyundrive_open.AliyundriveOpen, fileId string) {
	log.Infof("[%v] Delete Aliyun temp file: %v", ali.ID, fileId)
	_, err := ali.Request("/adrive/v1.0/openFile/delete", http.MethodPost, func(req *resty.Request) {
		req.SetBody(base.Json{
			"drive_id": ali.DriveId,
			"file_id":  fileId,
		})
	})
	if err != nil {
		log.Warnf("删除文件%v失败： %v", fileId, err)
	}
}

func (d *AliyundriveShare2Open) request(ali *aliyundrive_open.AliyundriveOpen, url, method string, callback base.ReqCallback) ([]byte, error) {
	var e ErrorResp
	req := base.RestyClient.R().
		SetError(&e).
		SetHeader("content-type", "application/json").
		SetHeader("Referer", "https://www.alipan.com/").
		SetHeader("User-Agent", conf.UserAgent).
		SetHeader("Authorization", "Bearer\t"+ali.AccessToken2).
		SetHeader(CanaryHeaderKey, CanaryHeaderValue).
		SetHeader("x-share-token", d.ShareToken)
	if callback != nil {
		callback(req)
	} else {
		req.SetBody("{}")
	}
	resp, err := req.Execute(method, url)
	if err != nil {
		log.Warnf("请求失败: %v", err)
		return nil, err
	}
	if e.Code != "" {
		log.Warnf("请求失败: %v %v", e.Code, e.Message)
		if e.Code == "AccessTokenInvalid" || e.Code == "ShareLinkTokenInvalid" {
			if e.Code == "AccessTokenInvalid" {
				err = ali.RefreshAliToken(true)
			} else {
				err = d.getShareToken()
			}
			if err != nil {
				return nil, err
			}
			return d.request(ali, url, method, callback)
		} else {
			return nil, errors.New(e.Code + ": " + e.Message)
		}
	}
	return resp.Body(), nil
}

func (d *AliyundriveShare2Open) getFiles(fileId string) ([]File, error) {
	files := make([]File, 0)
	data := base.Json{
		"limit":           200,
		"order_by":        d.OrderBy,
		"order_direction": d.OrderDirection,
		"parent_file_id":  fileId,
		"share_id":        d.ShareId,
		"marker":          "",
	}
	retry := 0
	for {
		var e ErrorResp
		var resp ListResp
		res, err := base.RestyClient.R().
			SetHeader("x-share-token", d.ShareToken).
			SetHeader(CanaryHeaderKey, CanaryHeaderValue).
			SetResult(&resp).SetError(&e).SetBody(data).
			Post("https://api.alipan.com/adrive/v3/file/list")
		if err != nil {
			return nil, err
		}
		log.Debugf("aliyundrive share get files: %s", res.String())
		if e.Code != "" {
			log.Warnf("aliyundrive share get files error: %v", e)
			if e.Code == "AccessTokenInvalid" || e.Code == "ShareLinkTokenInvalid" {
				err = d.getShareToken()
				if err != nil {
					return nil, err
				}
				return d.getFiles(fileId)
			}
			if e.Code != "ParamFlowException" || retry > 9 {
				return nil, errors.New(e.Message)
			}
		}
		if e.Code == "" {
			data["marker"] = resp.NextMarker
			files = append(files, resp.Items...)
			if resp.NextMarker == "" {
				break
			}
		} else {
			retry++
			log.Infof("retry get files: %v", retry)
		}
	}
	return files, nil
}

func (d *AliyundriveShare2Open) saveTo115(ctx context.Context, pan115 *_115.Pan115, file model.Obj, link *model.Link, args model.LinkArgs) (*model.Link, error) {
	if ok, err := pan115.UploadAvailable(); err != nil || !ok {
		return nil, err
	}
	log.Debugf("save file to 115 cloud: file=%v dir=%v", file.GetID(), pan115.TempDirId)
	fs := stream.FileStream{
		Obj: file,
		Ctx: ctx,
	}

	ss, err := stream.NewSeekableStream(fs, link)
	if err != nil {
		log.Warnf("NewSeekableStream failed: %v", err)
		return link, err
	}

	preHash := "2EF7BDE608CE5404E97D5F042F95F89F1C232871"
	fullHash := fs.GetHash().GetHash(utils.SHA1)
	log.Infof("id=%v name=%v size=%v hash=%v", fs.GetID(), fs.GetName(), fs.GetSize(), fullHash)
	res, err := pan115.RapidUpload(fs.GetSize(), fs.GetName(), pan115.TempDirId, preHash, fullHash, ss)
	if err != nil {
		log.Warnf("115 upload failed: %v", err)
		return link, nil
	}
	log.Debugf("115.RapidUpload: %v", res)
	for i := 0; i < 5; i++ {
		var f = &_115.FileObj{
			File: driver.File{
				PickCode: res.PickCode,
			},
		}
		link115, err := pan115.Link(ctx, f, args)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		go d.delayDelete115(pan115, fullHash)
		log.Infof("使用115链接: %v", link115.URL)
		exp := 4 * time.Hour
		return &model.Link{
			URL:        link115.URL,
			Header:     link115.Header,
			Expiration: &exp,
		}, nil
	}
	return link, nil
}

func (d *AliyundriveShare2Open) delayDelete115(pan115 *_115.Pan115, fileId string) {
	delayTime := setting.GetInt(conf.DeleteDelayTime, 900)
	if delayTime == 0 {
		return
	}

	log.Infof("Delete 115 temp file %v after %v seconds.", fileId, delayTime)
	time.Sleep(time.Duration(delayTime) * time.Second)
	pan115.DeleteTempFile(fileId)
}

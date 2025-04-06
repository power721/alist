package aliyundrive_share2_open

import (
	"context"
	"errors"
	"fmt"
	"github.com/SheltonZhu/115driver/pkg/driver"
	_115 "github.com/alist-org/alist/v3/drivers/115"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/setting"
	"github.com/alist-org/alist/v3/internal/stream"
	"github.com/alist-org/alist/v3/internal/token"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/go-resty/resty/v2"
	"net/http"
	"strconv"
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

var RefreshTokenOpen = ""
var AccessTokenOpen = ""
var RefreshToken = ""
var AccessToken = ""

var lazyLoad = false
var initialized = false
var ClientID = ""
var ClientSecret = ""
var DriveId = ""
var ParentFileId = ""

var DelayTime int64 = 1500
var lastTime int64 = 0

var userid = ""
var nickname = ""
var cleaned = false

func (d *AliyundriveShare2Open) refreshOpenToken(force bool) error {
	accountId := setting.GetStr("ali_account_id", "1")
	accessTokenOpen := token.GetToken("AccessTokenOpen-"+accountId, 7200)
	refreshTokenOpen := token.GetToken("RefreshTokenOpen-"+accountId, 0)
	log.Debugf("force %v accountId %v accessTokenOpen %v refreshTokenOpen: %v", force, accountId, accessTokenOpen, refreshTokenOpen)
	if !force && accessTokenOpen != "" && refreshTokenOpen != "" {
		RefreshTokenOpen, AccessTokenOpen = refreshTokenOpen, accessTokenOpen
		log.Debugf("RefreshTokenOpen已经存在")
		return nil
	}
	if refreshTokenOpen != "" {
		RefreshTokenOpen = refreshTokenOpen
	}

	t := time.Now()
	url := setting.GetStr("open_token_url", d.base+"/oauth/access_token")
	log.Println("refreshOpenToken", accountId, url, force)
	//var resp base.TokenResp
	var e ErrorResp
	res, err := base.RestyClient.R().
		ForceContentType("application/json").
		SetBody(base.Json{
			"client_id":     ClientID,
			"client_secret": ClientSecret,
			"grant_type":    "refresh_token",
			"refresh_token": RefreshTokenOpen,
		}).
		//SetResult(&resp).
		SetError(&e).
		Post(url)
	if err != nil {
		return err
	}
	log.Debugf("[ali_open] refresh open token response: %s", res.String())
	if e.Code != "" {
		return fmt.Errorf("failed to refresh open token: %s", e.Message)
	}
	refresh, access := utils.Json.Get(res.Body(), "refresh_token").ToString(), utils.Json.Get(res.Body(), "access_token").ToString()
	if refresh == "" {
		return errors.New("failed to refresh open token: refresh token is empty")
	}
	log.Debugf("[ali_share_open] toekn exchange: %s -> %s", RefreshToken, refresh)
	RefreshTokenOpen, AccessTokenOpen = refresh, access

	if err = d.checkUserId(); err != nil {
		RefreshTokenOpen = refreshTokenOpen
		AccessTokenOpen = accessTokenOpen
		return err
	}

	d.SaveOpenToken(t)

	return nil
}

func (d *AliyundriveShare2Open) SaveOpenToken(t time.Time) {
	accountId := setting.GetInt("ali_account_id", 1)
	item := &model.Token{
		Key:       "AccessTokenOpen-" + strconv.Itoa(accountId),
		Value:     AccessTokenOpen,
		AccountId: accountId,
		Modified:  t,
	}

	err := token.SaveToken(item)
	if err != nil {
		log.Warnf("save AccessTokenOpen failed: %v", err)
	}

	item = &model.Token{
		Key:       "RefreshTokenOpen-" + strconv.Itoa(accountId),
		Value:     RefreshTokenOpen,
		AccountId: accountId,
		Modified:  t,
	}

	err = token.SaveToken(item)
	if err != nil {
		log.Warnf("save RefreshTokenOpen failed: %v", err)
	}
}

func (d *AliyundriveShare2Open) refreshToken(force bool) error {
	accountId := setting.GetStr("ali_account_id", "1")
	accessToken := token.GetToken("AccessToken-"+accountId, 7200)
	refreshToken := token.GetToken("RefreshToken-"+accountId, 0)
	log.Debugf("refreshToken: %v %v %v", accountId, accessToken, refreshToken)
	if !force && accessToken != "" && refreshToken != "" {
		RefreshToken, AccessToken = refreshToken, accessToken
		log.Debugf("RefreshToken已经存在")
		return nil
	}
	if refreshToken != "" {
		RefreshToken = refreshToken
	}

	if lastTime > 0 {
		diff := lastTime + DelayTime - time.Now().UnixMilli()
		time.Sleep(time.Duration(diff) * time.Millisecond)
	}

	t := time.Now()
	url := "https://auth.alipan.com/v2/account/token"
	log.Println("refreshToken", accountId, url)
	var resp base.TokenResp
	var e ErrorResp
	_, err := base.RestyClient.R().
		SetBody(base.Json{"refresh_token": RefreshToken, "grant_type": "refresh_token"}).
		SetResult(&resp).
		SetError(&e).
		Post(url)
	if err != nil {
		return err
	}
	if e.Code != "" {
		return fmt.Errorf("failed to refresh ali token: %s", e.Message)
	}
	RefreshToken, AccessToken = resp.RefreshToken, resp.AccessToken

	d.SaveToken(t)

	return nil
}

func (d *AliyundriveShare2Open) reloadUser() {
	userid = ""
	d.getUser()
}

func (d *AliyundriveShare2Open) getUser() {
	if userid == "" {
		res, err := d.request("https://user.aliyundrive.com/v2/user/get", http.MethodPost, nil)
		lastTime = time.Now().UnixMilli()
		if err != nil {
			log.Warnf("getUser error: %v", err)
			return
		}
		userid = utils.Json.Get(res, "user_id").ToString()
		nickname = utils.Json.Get(res, "nick_name").ToString()
		log.Printf("阿里token 账号(%v) 昵称： %v", userid, nickname)
	}
}

func (d *AliyundriveShare2Open) checkUserId() error {
	res, err := d.requestOpen("/adrive/v1.0/user/getDriveInfo", http.MethodPost, nil)
	lastTime = time.Now().UnixMilli()
	if err != nil {
		log.Warnf("getDriveInfo error: %v", err)
		return err
	}
	uid := utils.Json.Get(res, "user_id").ToString()
	name := utils.Json.Get(res, "name").ToString()
	log.Printf("开放token 账号(%v) 昵称： %v", uid, name)
	if uid != userid {
		d.reloadUser()
		return errors.New("阿里Token与开放Token账号不匹配！")
	}
	DriveId = utils.Json.Get(res, "resource_drive_id").ToString()
	return nil
}

func (d *AliyundriveShare2Open) getDriveId() {
	if DriveId == "" {
		res, err := d.requestOpen("/adrive/v1.0/user/getDriveInfo", http.MethodPost, nil)
		lastTime = time.Now().UnixMilli()
		if err != nil {
			log.Warnf("getDriveId error: %v", err)
			return
		}
		uid := utils.Json.Get(res, "user_id").ToString()
		name := utils.Json.Get(res, "name").ToString()
		log.Printf("开放token 账号(%v) 昵称： %v", uid, name)
		DriveId = utils.Json.Get(res, "resource_drive_id").ToString()
		if DriveId == "" {
			DriveId = utils.Json.Get(res, "default_drive_id").ToString()
			log.Printf("备份盘ID： %v", DriveId)
		} else {
			log.Printf("资源盘ID： %v", DriveId)
		}
	}
}

func (d *AliyundriveShare2Open) createFolderOpen() {
	if ParentFileId != "" {
		return
	}

	res, err := d.requestOpen("/adrive/v1.0/openFile/create", http.MethodPost, func(req *resty.Request) {
		req.SetBody(base.Json{
			"check_name_mode": "refuse",
			"drive_id":        DriveId,
			"name":            "xiaoya-tvbox-temp",
			"parent_file_id":  "root",
			"type":            "folder",
		})
	})

	if err != nil {
		log.Warnf("创建缓存文件夹失败: %v", err)
	} else {
		ParentFileId = utils.Json.Get(res, "file_id").ToString()
	}

	if ParentFileId == "" {
		ParentFileId = "root"
	}
	log.Printf("缓存文件夹ID： %v", ParentFileId)
}

func (d *AliyundriveShare2Open) SaveToken(t time.Time) {
	accountId := setting.GetInt("ali_account_id", 1)
	item := &model.Token{
		Key:       "AccessToken-" + strconv.Itoa(accountId),
		Value:     AccessToken,
		AccountId: accountId,
		Modified:  t,
	}

	err := token.SaveToken(item)
	if err != nil {
		log.Printf("save AccessToken failed: %v", err)
	}

	if RefreshToken == "" {
		return
	}

	item = &model.Token{
		Key:       "RefreshToken-" + strconv.Itoa(accountId),
		Value:     RefreshToken,
		AccountId: accountId,
		Modified:  t,
	}

	err = token.SaveToken(item)
	if err != nil {
		log.Printf("save RefreshToken failed: %v", err)
	}
}

// do others that not defined in Driver interface
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
	lastTime = time.Now().UnixMilli()
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

func (d *AliyundriveShare2Open) saveFile(fileId string) (string, error) {
	data := base.Json{
		"requests": []base.Json{
			{
				"body": base.Json{
					"file_id":           fileId,
					"share_id":          d.ShareId,
					"auto_rename":       true,
					"to_parent_file_id": ParentFileId,
					"to_drive_id":       DriveId,
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

	res, err := d.request("https://api.alipan.com/adrive/v4/batch", http.MethodPost, func(req *resty.Request) {
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

func (d *AliyundriveShare2Open) getOpenLink(file model.Obj) (*model.Link, string, error) {
	res, err := d.requestOpen("/adrive/v1.0/openFile/getDownloadUrl", http.MethodPost, func(req *resty.Request) {
		req.SetBody(base.Json{
			"drive_id":   DriveId,
			"file_id":    file.GetID(),
			"expire_sec": 14400,
		})
	})

	go d.deleteDelay(file.GetID())

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

func (d *AliyundriveShare2Open) getDownloadUrl(fileId string) (string, string, error) {
	log.Infof("getDownloadUrl %v %v", DriveId, fileId)
	res, err := d.requestOpen("/adrive/v1.0/openFile/getDownloadUrl", http.MethodPost, func(req *resty.Request) {
		req.SetBody(base.Json{
			"drive_id":   DriveId,
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

func (d *AliyundriveShare2Open) getPreviewLink(file model.Obj) (*model.Link, error) {
	res, err := d.requestOpen("/adrive/v1.0/openFile/getVideoPreviewPlayInfo", http.MethodPost, func(req *resty.Request) {
		req.SetBody(base.Json{
			"drive_id":       DriveId,
			"file_id":        file.GetID(),
			"category":       "live_transcoding",
			"template_id":    "",
			"mode":           "high_res",
			"url_expire_sec": 14400,
		})
	})

	go d.deleteDelay(file.GetID())

	if err != nil {
		log.Errorf("getPreviewLink failed: %v", err)
		return nil, err
	}

	var resp VideoPreviewResponse
	err = utils.Json.Unmarshal(res, &resp)
	if err != nil {
		log.Errorf("Unmarshal failed: %v", err)
		return nil, err
	}

	log.Infof("%v", resp)

	url := ""
	for _, item := range resp.PlayInfo.Videos {
		if item.Status == "finished" {
			url = item.Url
		}
	}

	if url == "" {
		url, _, _ = d.getDownloadUrl(file.GetID())
	}

	exp := 895 * time.Second
	return &model.Link{
		URL:        url,
		Expiration: &exp,
	}, nil
}

func (d *AliyundriveShare2Open) getLink(file model.Obj) (*model.Link, error) {
	res, err := d.request("https://api.alipan.com/v2/file/get_video_preview_play_info", http.MethodPost, func(req *resty.Request) {
		req.SetBody(base.Json{
			"drive_id":       DriveId,
			"file_id":        file.GetID(),
			"category":       "live_transcoding",
			"template_id":    "",
			"mode":           "high_res",
			"url_expire_sec": 14400,
		})
	})

	go d.deleteDelay(file.GetID())

	if err != nil {
		log.Errorf("getLink failed: %v", err)
		return nil, err
	}
	var resp VideoPreviewResponse
	err = utils.Json.Unmarshal(res, &resp)
	if err != nil {
		log.Errorf("Unmarshal failed: %v", err)
		return nil, err
	}

	log.Debugf("%v", resp)

	url := ""
	for _, item := range resp.PlayInfo.Videos {
		if item.Status == "finished" {
			url = item.Url
		}
	}

	exp := time.Hour
	return &model.Link{
		URL:        url,
		Expiration: &exp,
	}, nil
}

func (d *AliyundriveShare2Open) deleteDelay(fileId string) {
	delayTime := setting.GetInt(conf.DeleteDelayTime, 900)
	if delayTime == 0 {
		return
	}

	log.Infof("Delete file %v after %v seconds.", fileId, delayTime)
	time.Sleep(time.Duration(delayTime) * time.Second)
	d.deleteOpen(fileId)
}

func (d *AliyundriveShare2Open) deleteOpen(fileId string) {
	_, err := d.requestOpen("/adrive/v1.0/openFile/delete", http.MethodPost, func(req *resty.Request) {
		req.SetBody(base.Json{
			"drive_id": DriveId,
			"file_id":  fileId,
		})
	})
	if err != nil {
		log.Warnf("删除文件%v失败： %v", fileId, err)
	}
}

func (d *AliyundriveShare2Open) delete(fileId string) error {
	data := base.Json{
		"requests": []base.Json{
			{
				"body": base.Json{
					"drive_id": DriveId,
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

	_, err := d.request("https://api.alipan.com/v4/batch", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data)
	})
	if err != nil {
		log.Warnf("删除文件%v失败： %v", fileId, err)
	}

	return err
}

func (d *AliyundriveShare2Open) request(url, method string, callback base.ReqCallback) ([]byte, error) {
	var e ErrorResp
	req := base.RestyClient.R().
		SetError(&e).
		SetHeader("content-type", "application/json").
		SetHeader("Referer", "https://www.alipan.com/").
		SetHeader("User-Agent", conf.UserAgent).
		SetHeader("Authorization", "Bearer\t"+AccessToken).
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
				err = d.refreshToken(true)
			} else {
				err = d.getShareToken()
			}
			if err != nil {
				return nil, err
			}
			return d.request(url, method, callback)
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

func (d *AliyundriveShare2Open) clean() {
	if cleaned || ParentFileId == "root" {
		return
	}

	cleaned = true
	files, err := d.listFiles(ParentFileId)
	if err != nil {
		log.Errorf("获取文件列表失败 %v", err)
		return
	}

	for _, file := range files {
		log.Infof("删除文件 %v %v 创建于 %v", file.Name, file.FileId, file.CreatedAt.Local())
		d.deleteOpen(file.FileId)
	}
}

func (d *AliyundriveShare2Open) listFiles(fileId string) ([]File, error) {
	marker := "first"
	res := make([]File, 0)
	for marker != "" {
		if marker == "first" {
			marker = ""
		}
		data := base.Json{
			"drive_id":        DriveId,
			"limit":           200,
			"marker":          marker,
			"order_by":        "created_at",
			"order_direction": "ASC",
			"parent_file_id":  fileId,
		}
		var resp ListResp
		_, err := d.requestOpen("/adrive/v1.0/openFile/list", http.MethodPost, func(req *resty.Request) {
			req.SetBody(data).SetResult(&resp)
		})
		if err != nil {
			return nil, err
		}
		marker = resp.NextMarker
		res = append(res, resp.Items...)
	}
	return res, nil
}

func (d *AliyundriveShare2Open) requestOpen(uri, method string, callback base.ReqCallback, retry ...bool) ([]byte, error) {
	b, err, _ := d.requestReturnErrResp(uri, method, callback, retry...)
	return b, err
}

func (d *AliyundriveShare2Open) requestReturnErrResp(uri, method string, callback base.ReqCallback, retry ...bool) ([]byte, error, *ErrorResp) {
	req := base.RestyClient.R()
	// TODO check whether access_token is expired
	req.SetHeader("Authorization", "Bearer "+AccessTokenOpen)
	if method == http.MethodPost {
		req.SetHeader("Content-Type", "application/json")
	}
	if callback != nil {
		callback(req)
	}
	var e ErrorResp
	req.SetError(&e)
	res, err := req.Execute(method, d.base+uri)
	if err != nil {
		if res != nil {
			log.Errorf("[aliyundrive_open] request error: %s", res.String())
		}
		return nil, err, nil
	}
	isRetry := len(retry) > 0 && retry[0]
	if e.Code != "" {
		if !isRetry && (utils.SliceContains([]string{"AccessTokenInvalid", "AccessTokenExpired", "I400JD"}, e.Code) || AccessTokenOpen == "") {
			err = d.refreshOpenToken(true)
			if err != nil {
				return nil, err, nil
			}
			return d.requestReturnErrResp(uri, method, callback, true)
		}
		return nil, fmt.Errorf("%s:%s", e.Code, e.Message), &e
	}
	return res.Body(), nil, nil
}

func (d *AliyundriveShare2Open) saveTo115(ctx context.Context, pan115 *_115.Pan115, file model.Obj, link *model.Link, args model.LinkArgs) (*model.Link, error) {
	if ok, err := pan115.UploadAvailable(); err != nil || !ok {
		return nil, err
	}
	log.Debugf("save file to 115 cloud: file=%v dir=%v", file.GetID(), _115.TempDirId)
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
	res, err := pan115.RapidUpload(fs.GetSize(), fs.GetName(), _115.TempDirId, preHash, fullHash, ss)
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

	log.Infof("Delete 115 file %v after %v seconds.", fileId, delayTime)
	time.Sleep(time.Duration(delayTime) * time.Second)
	pan115.DeleteTempFile(fileId)
}

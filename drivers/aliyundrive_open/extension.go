package aliyundrive_open

import (
	"context"
	"errors"
	"fmt"
	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/token"
	"github.com/alist-org/alist/v3/pkg/utils"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
	"time"
)

const (
	// CanaryHeaderKey CanaryHeaderValue for lifting rate limit restrictions
	CanaryHeaderKey   = "X-Canary"
	CanaryHeaderValue = "client=web,app=share,version=v2.3.1"
)

type ErrorResp struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (d *AliyundriveOpen) Request(uri, method string, callback base.ReqCallback, retry ...bool) ([]byte, error) {
	return d.request(uri, method, callback, retry...)
}

func (d *AliyundriveOpen) RefreshAliToken(force bool) error {
	accountId := strconv.Itoa(d.AccountId)
	accessToken := token.GetToken("AccessToken-"+accountId, 7200)
	refreshToken := token.GetToken("RefreshToken-"+accountId, 0)
	log.Debugf("refreshToken: %v %v %v", accountId, accessToken, refreshToken)
	if !force && accessToken != "" && refreshToken != "" {
		d.RefreshToken2, d.AccessToken2 = refreshToken, accessToken
		log.Debugf("RefreshToken已经存在")
		return nil
	}
	if refreshToken != "" {
		d.RefreshToken2 = refreshToken
	}

	t := time.Now()
	url := "https://auth.alipan.com/v2/account/token"
	log.Println("refreshToken", accountId, url)
	var resp base.TokenResp
	var e ErrorResp
	_, err := base.RestyClient.R().
		SetBody(base.Json{"refresh_token": d.RefreshToken2, "grant_type": "refresh_token"}).
		SetResult(&resp).
		SetError(&e).
		Post(url)
	if err != nil {
		return err
	}
	if e.Code != "" {
		return fmt.Errorf("failed to refresh ali token: %s", e.Message)
	}
	d.RefreshToken2, d.AccessToken2 = resp.RefreshToken, resp.AccessToken

	d.SaveToken(t)

	return nil
}

func (d *AliyundriveOpen) SaveToken(t time.Time) {
	accountId := d.AccountId
	item := &model.Token{
		Key:       "AccessToken-" + strconv.Itoa(accountId),
		Value:     d.AccessToken2,
		AccountId: accountId,
		Modified:  t,
	}

	err := token.SaveToken(item)
	if err != nil {
		log.Printf("save AccessToken failed: %v", err)
	}

	if d.RefreshToken2 == "" {
		return
	}

	item = &model.Token{
		Key:       "RefreshToken-" + strconv.Itoa(accountId),
		Value:     d.RefreshToken2,
		AccountId: accountId,
		Modified:  t,
	}

	err = token.SaveToken(item)
	if err != nil {
		log.Printf("save RefreshToken failed: %v", err)
	}
}

func (d *AliyundriveOpen) SaveOpenToken(t time.Time) {
	accountId := strconv.Itoa(d.AccountId)
	item := &model.Token{
		Key:       "AccessTokenOpen-" + accountId,
		Value:     d.AccessToken,
		AccountId: d.AccountId,
		Modified:  t,
	}

	err := token.SaveToken(item)
	if err != nil {
		log.Warnf("save AccessTokenOpen failed: %v", err)
	}

	item = &model.Token{
		Key:       "RefreshTokenOpen-" + accountId,
		Value:     d.RefreshToken,
		AccountId: d.AccountId,
		Modified:  t,
	}

	err = token.SaveToken(item)
	if err != nil {
		log.Warnf("save RefreshTokenOpen failed: %v", err)
	}
}

func (d *AliyundriveOpen) createTempDir(ctx context.Context) {
	dir := &model.Object{
		ID:   "root",
		Path: "",
	}

	res, err := d.MakeDir(ctx, dir, conf.TempDirName)

	if err != nil {
		log.Warnf("[%v] 创建阿里缓存文件夹失败: %v", d.AccountId, err)
	} else {
		d.TempDirId = res.GetID()
	}

	if d.TempDirId == "" {
		d.TempDirId = "root"
	}
	log.Printf("[%v] 阿里缓存文件夹ID： %v", d.AccountId, d.TempDirId)

	d.cleanTempFolder(ctx)
}

func (d *AliyundriveOpen) cleanTempFolder(ctx context.Context) {
	if d.TempDirId == "root" {
		return
	}

	dir := &model.Object{
		ID:   d.TempDirId,
		Path: "",
	}

	files, err := d.List(ctx, dir, model.ListArgs{})
	if err != nil {
		log.Errorf("获取文件列表失败 %v", err)
		return
	}

	for _, file := range files {
		log.Infof("删除文件 %v %v", file.GetName(), file.GetID())
		f := &model.Object{
			ID: file.GetID(),
		}
		_ = d.Remove(ctx, f)
	}
}

func (d *AliyundriveOpen) checkUserId() error {
	userid, err := d.getUser()
	if err != nil {
		return err
	}

	res, err := d.request("/adrive/v1.0/user/getDriveInfo", http.MethodPost, nil)
	if err != nil {
		log.Warnf("getDriveInfo error: %v", err)
		return err
	}
	uid := utils.Json.Get(res, "user_id").ToString()
	name := utils.Json.Get(res, "name").ToString()
	log.Printf("[%v] 开放token 账号(%v) 昵称： %v", d.AccountId, uid, name)
	if uid != userid {
		return errors.New("阿里Token与开放Token账号不匹配！")
	}
	d.DriveId = utils.Json.Get(res, d.DriveType+"_drive_id").ToString()
	if d.DriveId == "" {
		d.DriveId = utils.Json.Get(res, "default_drive_id").ToString()
		log.Infof("[%v] use default drive: %v", d.AccountId, d.DriveId)
	} else {
		log.Infof("[%v] use %v drive: %v", d.AccountId, d.DriveType, d.DriveId)
	}
	return nil
}

func (d *AliyundriveOpen) getUser() (string, error) {
	res, err := d.request2("https://user.aliyundrive.com/v2/user/get", http.MethodPost, nil)
	if err != nil {
		return "", err
	}
	userid := utils.Json.Get(res, "user_id").ToString()
	nickname := utils.Json.Get(res, "nick_name").ToString()
	log.Printf("[%v] 阿里token 账号(%v) 昵称： %v", d.AccountId, userid, nickname)
	return userid, nil
}

func (d *AliyundriveOpen) request2(url, method string, callback base.ReqCallback) ([]byte, error) {
	var e ErrorResp
	req := base.RestyClient.R().
		SetError(&e).
		SetHeader("content-type", "application/json").
		SetHeader("Referer", "https://www.alipan.com/").
		SetHeader("User-Agent", conf.UserAgent).
		SetHeader("Authorization", "Bearer\t"+d.AccessToken2).
		SetHeader(CanaryHeaderKey, CanaryHeaderValue)
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
				err = d.RefreshAliToken(true)
			}
			if err != nil {
				return nil, err
			}
			return d.request2(url, method, callback)
		} else {
			return nil, errors.New(e.Code + ": " + e.Message)
		}
	}
	return resp.Body(), nil
}

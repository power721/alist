package aliyundrive_share2_open

import (
	"errors"
	"fmt"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/setting"
	"github.com/alist-org/alist/v3/internal/token"
	"github.com/alist-org/alist/v3/pkg/utils"
	"net/http"
	"strconv"
	"time"

	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/op"
	log "github.com/sirupsen/logrus"
)

const (
	// CanaryHeaderKey CanaryHeaderValue for lifting rate limit restrictions
	CanaryHeaderKey   = "X-Canary"
	CanaryHeaderValue = "client=web,app=share,version=v2.3.1"
)

func (d *AliyundriveShare2Open) refreshOpenToken(force bool) error {
	accountId := setting.GetStr("ali_account_id", "")
	accessTokenOpen := token.GetToken("AccessTokenOpen-" + accountId)
	refreshTokenOpen := token.GetToken("RefreshTokenOpen-" + accountId)
	utils.Log.Debugf("force %v accountID %v accessTokenOpen %v refreshTokenOpen: %v", force, accountId, accessTokenOpen, refreshTokenOpen)
	if !force && accessTokenOpen != "" && refreshTokenOpen != "" {
		d.RefreshTokenOpen, d.AccessTokenOpen = refreshTokenOpen, accessTokenOpen
		utils.Log.Println("RefreshTokenOpen已经存在")
		return nil
	}
	if refreshTokenOpen != "" {
	    d.RefreshTokenOpen = refreshTokenOpen
	}

	t := time.Now()
	url := setting.GetStr("open_token_url", d.base+"/oauth/access_token")
	if d.OauthTokenURL != "" && d.ClientID == "" {
		url = d.OauthTokenURL
	}
	utils.Log.Println("refreshOpenToken", accountId, url)
	//var resp base.TokenResp
	var e ErrorResp
	res, err := base.RestyClient.R().
		ForceContentType("application/json").
		SetBody(base.Json{
			"client_id":     d.ClientID,
			"client_secret": d.ClientSecret,
			"grant_type":    "refresh_token",
			"refresh_token": d.RefreshTokenOpen,
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
	d.RefreshTokenOpen, d.AccessTokenOpen = refresh, access

	d.SaveOpenToken(t)

	op.MustSaveDriverStorage(d)
	return nil
}

func (d *AliyundriveShare2Open) SaveOpenToken(t time.Time) {
	accountId := setting.GetInt("ali_account_id", 0)
	item := &model.Token{
		Key:       "AccessTokenOpen-" + strconv.Itoa(accountId),
		Value:     d.AccessTokenOpen,
		AccountId: accountId,
		Modified:  t,
	}

	err := token.SaveToken(item)
	if err != nil {
		utils.Log.Printf("save AccessTokenOpen failed: %v", err)
	}

	item = &model.Token{
		Key:       "RefreshTokenOpen-" + strconv.Itoa(accountId),
		Value:     d.RefreshTokenOpen,
		AccountId: accountId,
		Modified:  t,
	}

	err = token.SaveToken(item)
	if err != nil {
		utils.Log.Printf("save RefreshTokenOpen failed: %v", err)
	}
}

func (d *AliyundriveShare2Open) refreshToken(force bool) error {
	accountId := setting.GetStr("ali_account_id", "")
	accessToken := token.GetToken("AccessToken-" + accountId)
	refreshToken := token.GetToken("RefreshToken-" + accountId)
	if !force && accessToken != "" && refreshToken != "" {
		d.RefreshToken, d.AccessToken = refreshToken, accessToken
		utils.Log.Println("RefreshToken已经存在")
		return nil
	}
	if refreshToken != "" {
	    d.RefreshToken = refreshToken
	}

	t := time.Now()
	url := "https://auth.aliyundrive.com/v2/account/token"
	utils.Log.Println("refreshToken", accountId, url)
	var resp base.TokenResp
	var e ErrorResp
	_, err := base.RestyClient.R().
		SetBody(base.Json{"refresh_token": d.RefreshToken, "grant_type": "refresh_token"}).
		SetResult(&resp).
		SetError(&e).
		Post(url)
	if err != nil {
		return err
	}
	if e.Code != "" {
		return fmt.Errorf("failed to refresh token: %s", e.Message)
	}
	d.RefreshToken, d.AccessToken = resp.RefreshToken, resp.AccessToken

	d.SaveToken(t)

	op.MustSaveDriverStorage(d)
	return nil
}

func (d *AliyundriveShare2Open) getDriveId() {
	if DriveId == "" {
		res, err := d.requestOpen("/adrive/v1.0/user/getDriveInfo", http.MethodPost, nil)
		if err != nil {
			return
		}
		d.DriveId = utils.Json.Get(res, d.DriveType+"_drive_id").ToString()
		if d.DriveId == "" {
			d.DriveId = utils.Json.Get(res, "default_drive_id").ToString()
		}
		DriveId = d.DriveId
		utils.Log.Printf("资源盘ID： %v", d.DriveId)
	} else {
		d.DriveId = DriveId
	}
}

func (d *AliyundriveShare2Open) SaveToken(t time.Time) {
	accountId := setting.GetInt("ali_account_id", 0)
	item := &model.Token{
		Key:       "AccessToken-" + strconv.Itoa(accountId),
		Value:     d.AccessToken,
		AccountId: accountId,
		Modified:  t,
	}

	err := token.SaveToken(item)
	if err != nil {
		utils.Log.Printf("save AccessToken failed: %v", err)
	}

	if d.RefreshToken == "" {
		return
	}

	item = &model.Token{
		Key:       "RefreshToken-" + strconv.Itoa(accountId),
		Value:     d.RefreshToken,
		AccountId: accountId,
		Modified:  t,
	}

	err = token.SaveToken(item)
	if err != nil {
		utils.Log.Printf("save RefreshToken failed: %v", err)
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
		Post("https://api.aliyundrive.com/v2/share_link/get_share_token")
	if err != nil {
		return err
	}
	if e.Code != "" {
		return errors.New(e.Message)
	}
	d.ShareToken = resp.ShareToken
	utils.Log.Debug("getShareToken", d.ShareId, d.ShareToken)
	return nil
}

func (d *AliyundriveShare2Open) request(url, method string, callback base.ReqCallback) ([]byte, error) {
	var e ErrorResp
	req := base.RestyClient.R().
		SetError(&e).
		SetHeader("content-type", "application/json").
		SetHeader("Referer", "https://www.aliyundrive.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/113.0.0.0 Safari/537.36").
		SetHeader("Authorization", "Bearer\t"+d.AccessToken).
		SetHeader(CanaryHeaderKey, CanaryHeaderValue).
		SetHeader("x-share-token", d.ShareToken)
	if callback != nil {
		callback(req)
	} else {
		req.SetBody("{}")
	}
	resp, err := req.Execute(method, url)
	if err != nil {
		return nil, err
	}
	if e.Code != "" {
		utils.Log.Println(e)
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
		"image_thumbnail_process": "image/resize,w_160/format,jpeg",
		"image_url_process":       "image/resize,w_1920/format,jpeg",
		"limit":                   200,
		"order_by":                d.OrderBy,
		"order_direction":         d.OrderDirection,
		"parent_file_id":          fileId,
		"share_id":                d.ShareId,
		"video_thumbnail_process": "video/snapshot,t_1000,f_jpg,ar_auto,w_300",
		"marker":                  "first",
	}
	for data["marker"] != "" {
		if data["marker"] == "first" {
			data["marker"] = ""
		}
		var e ErrorResp
		var resp ListResp
		res, err := base.RestyClient.R().
			SetHeader("x-share-token", d.ShareToken).
			SetHeader(CanaryHeaderKey, CanaryHeaderValue).
			SetResult(&resp).SetError(&e).SetBody(data).
			Post("https://api.aliyundrive.com/adrive/v3/file/list")
		if err != nil {
			return nil, err
		}
		log.Debugf("aliyundrive share get files: %s", res.String())
		if e.Code != "" {
			if e.Code == "AccessTokenInvalid" || e.Code == "ShareLinkTokenInvalid" {
				err = d.getShareToken()
				if err != nil {
					return nil, err
				}
				return d.getFiles(fileId)
			}
			return nil, errors.New(e.Message)
		}
		data["marker"] = resp.NextMarker
		files = append(files, resp.Items...)
	}
	if len(files) > 0 && d.DriveId == "" {
		d.DriveId = files[0].DriveId
	}
	return files, nil
}

func (d *AliyundriveShare2Open) requestOpen(uri, method string, callback base.ReqCallback, retry ...bool) ([]byte, error) {
	b, err, _ := d.requestReturnErrResp(uri, method, callback, retry...)
	return b, err
}

func (d *AliyundriveShare2Open) requestReturnErrResp(uri, method string, callback base.ReqCallback, retry ...bool) ([]byte, error, *ErrorResp) {
	req := base.RestyClient.R()
	// TODO check whether access_token is expired
	req.SetHeader("Authorization", "Bearer "+d.AccessTokenOpen)
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
		if !isRetry && (utils.SliceContains([]string{"AccessTokenInvalid", "AccessTokenExpired", "I400JD"}, e.Code) || d.AccessTokenOpen == "") {
			err = d.refreshOpenToken(false)
			if err != nil {
				return nil, err, nil
			}
			return d.requestReturnErrResp(uri, method, callback, true)
		}
		return nil, fmt.Errorf("%s:%s", e.Code, e.Message), &e
	}
	return res.Body(), nil, nil
}

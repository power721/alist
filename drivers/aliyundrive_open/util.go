package aliyundrive_open

import (
	"context"
	"errors"
	"fmt"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/setting"
	"github.com/alist-org/alist/v3/internal/token"
	"net/http"
	"strconv"
	"time"

	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
)

// do others that not defined in Driver interface

func (d *AliyundriveOpen) refreshToken() error {
	accountId := strconv.Itoa(d.AccountId)
	accessTokenOpen := token.GetToken("AccessTokenOpen-" + accountId)
	refreshTokenOpen := token.GetToken("RefreshTokenOpen-" + accountId)
	utils.Log.Debugf("accountID %v accessTokenOpen %v refreshTokenOpen: %v", accountId, accessTokenOpen, refreshTokenOpen)
	if accessTokenOpen != "" && refreshTokenOpen != "" {
		d.RefreshToken, d.AccessToken = refreshTokenOpen, accessTokenOpen
		utils.Log.Println("RefreshTokenOpen已经存在")
		return nil
	}

	t := time.Now()
	url := setting.GetStr("open_token_url", d.base+"/oauth/access_token")
	if d.OauthTokenURL != "" && d.ClientID == "" {
		url = d.OauthTokenURL
	}
	utils.Log.Println("refreshOpenToken", url)
	//var resp base.TokenResp
	var e ErrResp
	res, err := base.RestyClient.R().
		ForceContentType("application/json").
		SetBody(base.Json{
			"client_id":     d.ClientID,
			"client_secret": d.ClientSecret,
			"grant_type":    "refresh_token",
			"refresh_token": d.RefreshToken,
		}).
		//SetResult(&resp).
		SetError(&e).
		Post(url)
	if err != nil {
		return err
	}
	log.Debugf("[ali_open] refresh token response: %s", res.String())
	if e.Code != "" {
		return fmt.Errorf("failed to refresh token: %s", e.Message)
	}
	refresh, access := utils.Json.Get(res.Body(), "refresh_token").ToString(), utils.Json.Get(res.Body(), "access_token").ToString()
	if refresh == "" {
		return errors.New("failed to refresh token: refresh token is empty")
	}
	d.RefreshToken, d.AccessToken = refresh, access

	d.SaveOpenToken(t)

	op.MustSaveDriverStorage(d)
	return nil
}

func (d *AliyundriveOpen) SaveOpenToken(t time.Time) {
	accountId := strconv.Itoa(d.AccountId)
	item := &model.Token{
		Key:      "AccessTokenOpen-" + accountId,
		Value:    d.AccessToken,
		Modified: t,
	}

	err := token.SaveToken(item)
	if err != nil {
		utils.Log.Printf("save AccessTokenOpen failed: %v", err)
	}

	item = &model.Token{
		Key:      "RefreshTokenOpen-" + accountId,
		Value:    d.RefreshToken,
		Modified: t,
	}

	err = token.SaveToken(item)
	if err != nil {
		utils.Log.Printf("save RefreshTokenOpen failed: %v", err)
	}
}

func (d *AliyundriveOpen) request(uri, method string, callback base.ReqCallback, retry ...bool) ([]byte, error) {
	b, err, _ := d.requestReturnErrResp(uri, method, callback, retry...)
	return b, err
}

func (d *AliyundriveOpen) requestReturnErrResp(uri, method string, callback base.ReqCallback, retry ...bool) ([]byte, error, *ErrResp) {
	req := base.RestyClient.R()
	// TODO check whether access_token is expired
	req.SetHeader("Authorization", "Bearer "+d.AccessToken)
	if method == http.MethodPost {
		req.SetHeader("Content-Type", "application/json")
	}
	if callback != nil {
		callback(req)
	}
	var e ErrResp
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
		if !isRetry && (utils.SliceContains([]string{"AccessTokenInvalid", "AccessTokenExpired", "I400JD"}, e.Code) || d.AccessToken == "") {
			err = d.refreshToken()
			if err != nil {
				return nil, err, nil
			}
			return d.requestReturnErrResp(uri, method, callback, true)
		}
		return nil, fmt.Errorf("%s:%s", e.Code, e.Message), &e
	}
	return res.Body(), nil, nil
}

func (d *AliyundriveOpen) list(ctx context.Context, data base.Json) (*Files, error) {
	var resp Files
	_, err := d.request("/adrive/v1.0/openFile/list", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data).SetResult(&resp)
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (d *AliyundriveOpen) getFiles(ctx context.Context, fileId string) ([]File, error) {
	marker := "first"
	res := make([]File, 0)
	for marker != "" {
		if marker == "first" {
			marker = ""
		}
		data := base.Json{
			"drive_id":        d.DriveId,
			"limit":           200,
			"marker":          marker,
			"order_by":        d.OrderBy,
			"order_direction": d.OrderDirection,
			"parent_file_id":  fileId,
			//"category":              "",
			//"type":                  "",
			//"video_thumbnail_time":  120000,
			//"video_thumbnail_width": 480,
			//"image_thumbnail_width": 480,
		}
		resp, err := d.limitList(ctx, data)
		if err != nil {
			return nil, err
		}
		marker = resp.NextMarker
		res = append(res, resp.Items...)
	}
	return res, nil
}

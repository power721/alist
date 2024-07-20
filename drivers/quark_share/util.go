package quark_share

import (
	"errors"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/setting"
	"github.com/alist-org/alist/v3/pkg/cookie"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/go-resty/resty/v2"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alist-org/alist/v3/drivers/base"
	log "github.com/sirupsen/logrus"
)

const UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) quark-cloud-drive/2.5.20 Chrome/100.0.4896.160 Electron/18.3.5.4-b478491100 Safari/537.36 Channel/pckk_other_ch"
const Referer = "https://pan.quark.cn"

var Cookie = ""
var ParentFileId = "0"

func (d *QuarkShare) request(pathname string, method string, callback base.ReqCallback, resp interface{}) ([]byte, error) {
	u := "https://drive.quark.cn/1/clouddrive" + pathname
	req := base.RestyClient.R()
	req.SetHeaders(map[string]string{
		"Cookie":     Cookie,
		"Accept":     "application/json, text/plain, */*",
		"User-Agent": UA,
		"Referer":    Referer,
	})
	req.SetQueryParam("pr", "ucpro")
	req.SetQueryParam("fr", "pc")
	if callback != nil {
		callback(req)
	}
	if resp != nil {
		req.SetResult(resp)
	}
	var e Resp
	req.SetError(&e)
	res, err := req.Execute(method, u)
	if err != nil {
		return nil, err
	}
	__puus := cookie.GetCookie(res.Cookies(), "__puus")
	if __puus != nil {
		Cookie = cookie.SetStr(Cookie, "__puus", __puus.Value)
	}
	if e.Status >= 400 || e.Code != 0 {
		return nil, errors.New(e.Message)
	}
	return res.Body(), nil
}

func (d *QuarkShare) getTempFolder() {
	files, err := d.GetFiles("0")
	if err != nil {
		log.Warnf("get files error: %v", err)
	}

	for _, file := range files {
		if file.Name == "alist-tvbox-temp" {
			ParentFileId = file.ID
			return
		}
	}

	d.createTempFolder()
}

func (d *QuarkShare) createTempFolder() {
	data := base.Json{
		"dir_init_lock": false,
		"dir_path":      "",
		"file_name":     "alist-tvbox-temp",
		"pdir_fid":      "0",
	}
	res, err := d.request("/file", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data)
	}, nil)
	fid := utils.Json.Get(res, "data", "fid").ToString()
	if fid != "" {
		ParentFileId = fid
	}
	log.Infof("create folder: %v", string(res))
	if err != nil {
		log.Warnf("create folder error: %v", err)
	}
}

func (d *QuarkShare) cleanTempFolder() {
	if ParentFileId == "0" {
		return
	}

	files, err := d.GetFiles(ParentFileId)
	if err != nil {
		log.Warnf("get files error: %v", err)
	}

	for _, file := range files {
		go d.deleteFile(file.ID)
	}
}

func (d *QuarkShare) GetFiles(parent string) ([]File, error) {
	files := make([]File, 0)
	page := 1
	size := 100
	query := map[string]string{
		"pdir_fid":     parent,
		"_size":        strconv.Itoa(size),
		"_fetch_total": "1",
	}
	if d.OrderBy != "none" {
		query["_sort"] = "file_type:asc," + d.OrderBy + ":" + d.OrderDirection
	}
	for {
		query["_page"] = strconv.Itoa(page)
		var resp SortResp
		_, err := d.request("/file/sort", http.MethodGet, func(req *resty.Request) {
			req.SetQueryParams(query)
		}, &resp)
		if err != nil {
			return nil, err
		}
		files = append(files, resp.Data.List...)
		if page*size >= resp.Metadata.Total {
			break
		}
		page++
	}
	return files, nil
}

func (d *QuarkShare) getShareToken() error {
	data := base.Json{
		"pwd_id":   d.ShareId,
		"passcode": d.SharePwd,
	}
	var errRes Resp
	var resp ShareTokenResp
	res, err := d.request("/share/sharepage/token", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data)
	}, &resp)
	log.Debugf("getShareToken: %v %v", d.ShareId, string(res))
	if err != nil {
		return err
	}
	if errRes.Code != 0 {
		return errors.New(errRes.Message)
	}
	d.ShareToken = resp.Data.ShareToken
	log.Debugf("getShareToken: %v %v", d.ShareId, d.ShareToken)
	return nil
}

func (d *QuarkShare) saveFile(id string) (string, error) {
	s := strings.Split(id, "-")
	fileId := s[0]
	fileTokenId := s[1]
	data := base.Json{
		"fid_list":       []string{fileId},
		"fid_token_list": []string{fileTokenId},
		"to_pdir_fid":    ParentFileId,
		"pwd_id":         d.ShareId,
		"stoken":         d.ShareToken,
		"pdir_fid":       "0",
		"scene":          "link",
	}
	query := map[string]string{
		"pr":           "ucpro",
		"fr":           "pc",
		"uc_param_str": "",
		"__dt":         strconv.Itoa(rand.Int()),
		"__t":          strconv.FormatInt(time.Now().Unix(), 10),
	}
	var resp SaveResp
	res, err := d.request("/share/sharepage/save", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data).SetQueryParams(query)
	}, &resp)
	log.Debugf("saveFile: %v %v", id, string(res))
	if err != nil {
		log.Warnf("save file failed: %v", err)
		return "", err
	}
	if resp.Status != 200 {
		return "", errors.New(resp.Message)
	}
	taskId := resp.Data.TaskId
	log.Debugf("save file task id: %v", taskId)

	newFileId, err := d.getSaveTaskResult(taskId)
	if err != nil {
		return "", err
	}
	log.Debugf("new file id: %v", newFileId)

	return newFileId, nil
}

func (d *QuarkShare) getSaveTaskResult(taskId string) (string, error) {
	time.Sleep(500 * time.Millisecond)
	for retry := 1; retry <= 30; {
		query := map[string]string{
			"pr":           "ucpro",
			"fr":           "pc",
			"uc_param_str": "",
			"retry_index":  strconv.Itoa(retry),
			"task_id":      taskId,
			"__dt":         strconv.Itoa(rand.Int()),
			"__t":          strconv.FormatInt(time.Now().Unix(), 10),
		}
		var resp SaveTaskResp
		res, err := d.request("/task", http.MethodGet, func(req *resty.Request) {
			req.SetQueryParams(query)
		}, &resp)
		log.Debugf("getSaveTaskResult: %v %v", taskId, string(res))
		if err != nil {
			log.Warnf("get save task result failed: %v", err)
			return "", err
		}
		if resp.Status != 200 {
			return "", errors.New(resp.Message)
		}
		if len(resp.Data.SaveAs.Fid) > 0 {
			return resp.Data.SaveAs.Fid[0], nil
		}
		time.Sleep(500 * time.Millisecond)
		retry++
	}
	return "", errors.New("Get task result failed.")
}

func (d *QuarkShare) getDownloadUrl(fileId string) (*model.Link, error) {
	data := base.Json{
		"fids": []string{fileId},
	}
	var resp DownResp
	res, err := d.request("/file/download", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data)
	}, &resp)
	log.Debugf("getDownloadUrl: %v %v", fileId, string(res))

	if err != nil {
		return nil, err
	}

	go d.deleteDelay(fileId)

	url := resp.Data[0].DownloadUrl
	if url == "" {
		log.Infof("getDownloadUrl: %v", string(res))
		return nil, errors.New("Cannot get download url!")
	}
	exp := 8 * time.Hour
	return &model.Link{
		URL:        url,
		Expiration: &exp,
		Header: http.Header{
			"Cookie":     []string{Cookie},
			"Referer":    []string{Referer},
			"User-Agent": []string{UA},
		},
		Concurrency: 16,
		PartSize:    2 * utils.MB,
	}, nil
}

func (d *QuarkShare) deleteDelay(fileId string) {
	delayTime := setting.GetInt(conf.DeleteDelayTime, 900)
	if delayTime == 0 {
		return
	}

	log.Infof("Delete file %v after %v seconds.", fileId, delayTime)
	time.Sleep(time.Duration(delayTime) * time.Second)
	d.deleteFile(fileId)
}

func (d *QuarkShare) deleteFile(fileId string) error {
	data := base.Json{
		"action_type":  1,
		"exclude_fids": []string{},
		"filelist":     []string{fileId},
	}
	var resp PlayResp
	res, err := d.request("/file/delete", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data)
	}, &resp)
	log.Debugf("deleteFile: %v %v", fileId, string(res))
	if err != nil {
		log.Warnf("Delete file failed: %v %v", fileId, err)
		return err
	}
	if resp.Status != 200 {
		log.Warnf("Delete file failed: %v %v", fileId, resp.Message)
		return errors.New(resp.Message)
	}
	return nil
}

func (d *QuarkShare) getShareFiles(id string) ([]File, error) {
	s := strings.Split(id, "-")
	fileId := s[0]
	files := make([]File, 0)
	page := 1
	for {
		query := map[string]string{
			"pr":            "ucpro",
			"fr":            "pc",
			"pwd_id":        d.ShareId,
			"stoken":        d.ShareToken,
			"pdir_fid":      fileId,
			"force":         "0",
			"_page":         strconv.Itoa(page),
			"_size":         "50",
			"_fetch_banner": "0",
			"_fetch_share":  "0",
			"_fetch_total":  "1",
			"_sort":         "file_type:asc,updated_at:desc",
		}
		var resp ListResp
		res, err := d.request("/share/sharepage/detail", http.MethodGet, func(req *resty.Request) {
			req.SetQueryParams(query)
		}, &resp)
		log.Debugf("quark share get files: %s", string(res))
		if err != nil {
			if err.Error() == "分享的stoken过期" {
				d.getShareToken()
				return d.getShareFiles(id)
			}
			return nil, err
		}
		if resp.Message == "ok" {
			files = append(files, resp.Data.Files...)
			if len(files) >= resp.Metadata.Total {
				break
			}
			page++
		} else {
			if resp.Message == "分享的stoken过期" {
				d.getShareToken()
				return d.getShareFiles(id)
			}
			return nil, errors.New(resp.Message)
		}
	}

	return files, nil
}

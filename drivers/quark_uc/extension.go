package quark

import (
	"errors"
	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type PlayResp struct {
	Resp
	Data struct {
		VideoList []struct {
			Accessable bool `json:"accessable"`
			VideoInfo  struct {
				Success bool   `json:"success"`
				Url     string `json:"url"`
			} `json:"video_info"`
		} `json:"video_list"`
	} `json:"data"`
}

func (d *QuarkOrUC) getUserInfo() error {
	res, err := d.request("/member", http.MethodGet, nil, nil)
	if err != nil {
		return err
	}

	log.Debugf("member: %v", string(res))
	memberType := utils.Json.Get(res, "data", "member_type").ToString()
	d.vip = memberType != "NORMAL"
	log.Infof("member type: %v", memberType)
	return nil
}

func (d *QuarkOrUC) getPlayUrl(file model.Obj) (*model.Link, error) {
	log.Debugf("get play url: %v", file.GetID())
	data := base.Json{
		"fid":         file.GetID(),
		"resolutions": "normal,low,high,super,2k,4k",
		"supports":    "fmp4,m3u8",
	}
	var resp PlayResp
	_, err := d.request("/file/v2/play", http.MethodPost, func(req *resty.Request) {
		req.SetHeader("User-Agent", d.conf.ua).
			SetBody(data)
	}, &resp)
	if err != nil {
		return nil, err
	}

	log.Debugf("play url: %v", resp)
	for _, item := range resp.Data.VideoList {
		if item.Accessable && item.VideoInfo.Success {
			return &model.Link{
				URL: item.VideoInfo.Url,
				Header: http.Header{
					"Cookie":     []string{d.Cookie},
					"Referer":    []string{d.conf.referer},
					"User-Agent": []string{d.conf.ua},
				},
			}, nil
		}
	}

	return nil, errors.New("cannot get play url")
}

func (d *QuarkOrUC) getTempFolder() {
	files, err := d.GetFiles("0")
	if err != nil {
		log.Warnf("get files error: %v", err)
	}

	for _, file := range files {
		if file.FileName == conf.TempDirName {
			d.TempDirId = file.Fid
			log.Infof("%v temp folder id: %v", d.config.Name, d.TempDirId)
			d.cleanTempFolder()
			return
		}
	}

	d.createTempFolder()
}

func (d *QuarkOrUC) createTempFolder() {
	data := base.Json{
		"dir_init_lock": false,
		"dir_path":      "",
		"file_name":     conf.TempDirName,
		"pdir_fid":      "0",
	}
	res, err := d.request("/file", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data)
	}, nil)
	fid := utils.Json.Get(res, "data", "fid").ToString()
	if fid != "" {
		d.TempDirId = fid
	}
	log.Infof("create temp folder: %v", string(res))
	if err != nil {
		log.Warnf("create temp folder error: %v", err)
	}
}

func (d *QuarkOrUC) cleanTempFolder() {
	if d.TempDirId == "0" {
		return
	}

	files, err := d.GetFiles(d.TempDirId)
	if err != nil {
		log.Warnf("get files error: %v", err)
	}

	for _, file := range files {
		go d.deleteFile(file.Fid)
	}
}

func (d *QuarkOrUC) deleteFile(fileId string) {
	data := base.Json{
		"action_type":  1,
		"exclude_fids": []string{},
		"filelist":     []string{fileId},
	}
	res, err := d.request("/file/delete", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data)
	}, nil)
	log.Debugf("deleteFile: %v %v", fileId, string(res))
	if err != nil {
		log.Warnf("Delete file failed: %v %v", fileId, err)
	}
}

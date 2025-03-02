package thunder_share

import (
	"context"
	"errors"
	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/drivers/thunder_browser"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

const (
	API_URL               = "https://x-api-pan.xunlei.com/drive/v1"
	SHARE_API_URL         = API_URL + "/share"
	SHARE_DETAIL_API_URL  = API_URL + "/share/detail"
	SHARE_RESTORE_API_URL = API_URL + "/share/restore"
)

var ParentFileId = ""

func (d *ThunderShare) saveFile(ctx context.Context, file model.Obj) (string, error) {
	storage := op.GetFirstDriver("ThunderBrowser")
	thunder, ok := storage.(*thunder_browser.ThunderBrowser)
	if !ok {
		return "", errors.New("ThunderBrowser storage error")
	}

	data := base.Json{
		"file_ids":          []string{file.GetID()},
		"ancestor_ids":      []string{},
		"parent_id":         ParentFileId,
		"share_id":          d.ShareId,
		"pass_code_token":   d.ShareToken,
		"specify_parent_id": true,
	}

	log.Debugf("save file to folder %v", ParentFileId)
	_, err := thunder.Request(SHARE_RESTORE_API_URL, http.MethodPost, func(r *resty.Request) {
		r.SetBody(data)
	}, nil)
	if err != nil {
		return "", err
	}

	time.Sleep(500 * time.Millisecond)
	var args model.ListArgs
	var dir model.Obj = &thunder_browser.Files{
		ID:    ParentFileId,
		Space: "",
	}
	files, err := thunder.List(ctx, dir, args)
	if err != nil {
		return "", err
	}

	for _, f := range files {
		log.Debugf("file: %v %v", f.GetID(), f.GetName())
		if f.GetName() == file.GetName() {
			return f.GetID(), nil
		}
	}

	return "", errors.New("file not found")
}

func (d *ThunderShare) getDownloadUrl(ctx context.Context, fileId string) (*model.Link, error) {
	var link *model.Link
	var args model.LinkArgs
	storage := op.GetFirstDriver("ThunderBrowser")
	thunder, ok := storage.(*thunder_browser.ThunderBrowser)
	if !ok {
		return link, errors.New("ThunderBrowser storage error")
	}

	var file model.Obj = &thunder_browser.Files{
		ID:    fileId,
		Space: "",
	}

	go d.deleteFile(ctx, thunder, file)

	log.Debugf("get link: %v", fileId)
	return thunder.Link(ctx, file, args)
}

func (d *ThunderShare) deleteFile(ctx context.Context, thunder *thunder_browser.ThunderBrowser, file model.Obj) {
	err := thunder.Remove(ctx, file)
	if err != nil {
		log.Warnf("delete temp file error: %v", err)
	}
}

func (t *ThunderShare) listShareFiles(ctx context.Context, dir model.Obj) ([]model.Obj, error) {
	storage := op.GetFirstDriver("ThunderBrowser")
	thunder, ok := storage.(*thunder_browser.ThunderBrowser)
	if !ok {
		return nil, errors.New("ThunderBrowser storage error")
	}
	files := make([]model.Obj, 0)

	parentId := dir.GetID()
	if parentId == "" {
		share, err := t.getShareInfo(ctx, thunder)
		if err != nil {
			return nil, err
		}
		for i := range share.Files {
			files = append(files, &share.Files[i])
		}
		return files, nil
	}

	pageToken := ""
	for {
		var fileList thunder_browser.FileList
		params := map[string]string{
			"share_id":        t.ShareId,
			"parent_id":       parentId,
			"pass_code_token": t.ShareToken,
			"page_token":      pageToken,
			"limit":           "100",
			"thumbnail_size":  "SIZE_SMALL",
		}

		_, err := thunder.Request(SHARE_DETAIL_API_URL, http.MethodGet, func(r *resty.Request) {
			r.SetContext(ctx)
			r.SetQueryParams(params)
		}, &fileList)
		if err != nil {
			return nil, err
		}

		for i := range fileList.Files {
			// 解决 "迅雷云盘" 重复出现问题————迅雷后端发送错误
			if fileList.Files[i].FolderType == "DEFAULT_ROOT" && fileList.Files[i].ID == "" && fileList.Files[i].Space == "" && dir.GetID() != "" {
				continue
			}
			files = append(files, &fileList.Files[i])
		}

		if fileList.NextPageToken == "" {
			break
		}
		pageToken = fileList.NextPageToken
	}
	return files, nil
}

func (t *ThunderShare) getShareInfo(ctx context.Context, thunder *thunder_browser.ThunderBrowser) (ShareInfo, error) {
	var share ShareInfo
	err := thunder.GetShareCaptchaToken()
	if err != nil {
		return share, err
	}

	params := map[string]string{
		"share_id":        t.ShareId,
		"pass_code":       t.SharePwd,
		"limit":           "100",
		"pass_code_token": "",
		"page_token":      "",
		"thumbnail_size":  "SIZE_LARGE",
	}

	_, err = thunder.Request(SHARE_API_URL, http.MethodGet, func(r *resty.Request) {
		r.SetContext(ctx)
		r.SetQueryParams(params)
	}, &share)
	if err != nil {
		return share, err
	}

	log.Debugf("get share token: %v", share.Token)
	t.ShareToken = share.Token
	return share, nil
}

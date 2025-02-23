package _189_share

import (
	"context"
	"errors"
	"github.com/Xhofe/go-cache"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	"golang.org/x/time/rate"
	"path/filepath"
	"strconv"
	"time"
)

// do others that not defined in Driver interface

var shareTokenCache = cache.NewMemCache(cache.WithShards[ShareInfo](128))
var limiter = rate.NewLimiter(rate.Every(3000*time.Millisecond), 1)

func (d *Cloud189Share) getShareInfo(shareId, pwd string) (ShareInfo, error) {

	tempShareInfo, exist := shareTokenCache.Get(shareId)
	if exist {
		return tempShareInfo, nil
	}

	var shareInfo ShareInfo
	_, err := d.client.R().SetQueryParam("shareCode", shareId).SetResult(&shareInfo).Get("https://cloud.189.cn/api/open/share/getShareInfoByCodeV2.action")

	if err != nil {
		utils.Log.Info("获取天翼网盘分享信息失败", err)
		return shareInfo, err
	}

	if shareInfo.ShareId == 0 {
		var checkShareInfo ShareInfo
		_, err = d.client.R().SetQueryParams(map[string]string{
			"shareCode":  shareId,
			"accessCode": pwd,
		}).SetResult(&checkShareInfo).Get("https://cloud.189.cn/api/open/share/checkAccessCode.action")
		if err != nil {
			utils.Log.Info("获取天翼网盘分享ID失败", err)
			return shareInfo, err
		}
		shareInfo.ShareId = checkShareInfo.ShareId
	}

	if shareInfo.FileId != "" {
		shareTokenCache.Set(shareId, shareInfo, cache.WithEx[ShareInfo](time.Minute*time.Duration(d.CacheExpiration)))
		return shareInfo, nil
	} else {
		utils.Log.Infof("获取天翼网盘分享信息为空:%v", shareInfo)
		return shareInfo, errors.New("获取天翼网盘分享信息为空")
	}

}

func (d *Cloud189Share) getShareFiles(ctx context.Context, virtualFile model.VirtualFile, dir model.Obj) ([]FileObj, error) {

	shareInfo, err := d.getShareInfo(virtualFile.ShareID, virtualFile.SharePwd)
	if err != nil {
		return nil, err
	}

	fileId := filepath.Base(dir.GetPath())
	if fileId == "0" {
		fileId = shareInfo.FileId
	}

	var res []FileObj
	for pageNum := 1; ; pageNum++ {

		var resp Cloud189FilesResp
		_, err := d.client.R().SetQueryParams(map[string]string{
			"pageNum":        strconv.Itoa(pageNum),
			"pageSize":       "60",
			"fileId":         fileId,
			"shareDirFileId": fileId,
			"isFolder":       strconv.FormatBool(shareInfo.IsFolder),
			"shareId":        strconv.Itoa(shareInfo.ShareId),
			"shareMode":      strconv.Itoa(shareInfo.ShareMode),
			"iconOption":     "5",
			"orderBy":        "filename",
			"descending":     "false",
			"accessCode":     virtualFile.SharePwd,
		}).SetResult(&resp).Get("https://cloud.189.cn/api/open/share/listShareDir.action")

		if err != nil {
			utils.Log.Infof("获取天翼云分享文件:%s失败: %v", dir.GetName(), err)
			return nil, err
		}

		for _, item := range resp.FileListAO.FileList {
			res = append(res, FileObj{
				ObjThumb: model.ObjThumb{
					Object: model.Object{
						ID:       string(item.ID),
						Name:     item.Name,
						Size:     item.Size,
						Ctime:    time.Time(item.CreateDate),
						Modified: time.Time(item.LastOpTime),
						IsFolder: false,
					},
					Thumbnail: model.Thumbnail{Thumbnail: item.Icon.SmallUrl},
				},
				oldName: item.Name,
			})
		}

		for _, item := range resp.FileListAO.FolderList {
			res = append(res, FileObj{
				ObjThumb: model.ObjThumb{
					Object: model.Object{
						ID:       string(item.ID),
						Name:     item.Name,
						Size:     0,
						Ctime:    time.Time(item.CreateDate),
						Modified: time.Time(item.LastOpTime),
						IsFolder: true,
					},
				},
				oldName: item.Name,
			})
		}

		// 获取完毕跳出
		if resp.FileListAO.Count == 0 {
			break
		}

	}
	return res, nil

}

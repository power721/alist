package _189pc

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/go-resty/resty/v2"
)

type Cloud189PC struct {
	model.Storage
	Addition

	identity string

	client *resty.Client

	loginParam *LoginParam
	tokenInfo  *AppSessionResp

	uploadThread int

	storageConfig driver.Config
}

func (y *Cloud189PC) Config() driver.Config {
	if y.storageConfig.Name == "" {
		y.storageConfig = config
	}
	return y.storageConfig
}

func (y *Cloud189PC) GetAddition() driver.Additional {
	return &y.Addition
}

func (y *Cloud189PC) Init(ctx context.Context) (err error) {
	// 兼容旧上传接口
	y.storageConfig.NoOverwriteUpload = y.isFamily() && (y.Addition.RapidUpload || y.Addition.UploadMethod == "old")

	// 处理个人云和家庭云参数
	if y.isFamily() && y.RootFolderID == "-11" {
		y.RootFolderID = ""
	}
	if !y.isFamily() && y.RootFolderID == "" {
		y.RootFolderID = "-11"
		y.FamilyID = ""
	}

	// 限制上传线程数
	y.uploadThread, _ = strconv.Atoi(y.UploadThread)
	if y.uploadThread < 1 || y.uploadThread > 32 {
		y.uploadThread, y.UploadThread = 3, "3"
	}

	// 初始化请求客户端
	if y.client == nil {
		y.client = base.NewRestyClient().SetHeaders(map[string]string{
			"Accept":  "application/json;charset=UTF-8",
			"Referer": WEB_URL,
		})
	}

	// 避免重复登陆
	identity := utils.GetMD5EncodeStr(y.Username + y.Password)
	if !y.isLogin() || y.identity != identity {
		y.identity = identity
		if err = y.login(); err != nil {
			return
		}
	}

	// 处理家庭云ID
	if y.isFamily() && y.FamilyID == "" {
		if y.FamilyID, err = y.getFamilyID(); err != nil {
			return err
		}
	}
	return
}

func (y *Cloud189PC) Drop(ctx context.Context) error {
	return nil
}

func (y *Cloud189PC) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	return y.getFiles(ctx, dir.GetID())
}

func (y *Cloud189PC) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	var downloadUrl struct {
		URL string `json:"fileDownloadUrl"`
	}

	fullUrl := API_URL
	if y.isFamily() {
		fullUrl += "/family/file"
	}
	fullUrl += "/getFileDownloadUrl.action"

	_, err := y.get(fullUrl, func(r *resty.Request) {
		r.SetContext(ctx)
		r.SetQueryParam("fileId", file.GetID())
		if y.isFamily() {
			r.SetQueryParams(map[string]string{
				"familyId": y.FamilyID,
			})
		} else {
			r.SetQueryParams(map[string]string{
				"dt":   "3",
				"flag": "1",
			})
		}
	}, &downloadUrl)
	if err != nil {
		return nil, err
	}

	// 重定向获取真实链接
	downloadUrl.URL = strings.Replace(strings.ReplaceAll(downloadUrl.URL, "&amp;", "&"), "http://", "https://", 1)
	res, err := base.NoRedirectClient.R().SetContext(ctx).SetDoNotParseResponse(true).Get(downloadUrl.URL)
	if err != nil {
		return nil, err
	}
	defer res.RawBody().Close()
	if res.StatusCode() == 302 {
		downloadUrl.URL = res.Header().Get("location")
	}

	like := &model.Link{
		URL: downloadUrl.URL,
		Header: http.Header{
			"User-Agent": []string{base.UserAgent},
		},
	}
	/*
		// 获取链接有效时常
		strs := regexp.MustCompile(`(?i)expire[^=]*=([0-9]*)`).FindStringSubmatch(downloadUrl.URL)
		if len(strs) == 2 {
			timestamp, err := strconv.ParseInt(strs[1], 10, 64)
			if err == nil {
				expired := time.Duration(timestamp-time.Now().Unix()) * time.Second
				like.Expiration = &expired
			}
		}
	*/
	return like, nil
}

func (y *Cloud189PC) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
	fullUrl := API_URL
	if y.isFamily() {
		fullUrl += "/family/file"
	}
	fullUrl += "/createFolder.action"

	var newFolder Cloud189Folder
	_, err := y.post(fullUrl, func(req *resty.Request) {
		req.SetContext(ctx)
		req.SetQueryParams(map[string]string{
			"folderName":   dirName,
			"relativePath": "",
		})
		if y.isFamily() {
			req.SetQueryParams(map[string]string{
				"familyId": y.FamilyID,
				"parentId": parentDir.GetID(),
			})
		} else {
			req.SetQueryParams(map[string]string{
				"parentFolderId": parentDir.GetID(),
			})
		}
	}, &newFolder)
	if err != nil {
		return nil, err
	}
	return &newFolder, nil
}

func (y *Cloud189PC) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	var resp CreateBatchTaskResp
	_, err := y.post(API_URL+"/batch/createBatchTask.action", func(req *resty.Request) {
		req.SetContext(ctx)
		req.SetFormData(map[string]string{
			"type": "MOVE",
			"taskInfos": MustString(utils.Json.MarshalToString(
				[]BatchTaskInfo{
					{
						FileId:   srcObj.GetID(),
						FileName: srcObj.GetName(),
						IsFolder: BoolToNumber(srcObj.IsDir()),
					},
				})),
			"targetFolderId": dstDir.GetID(),
		})
		if y.isFamily() {
			req.SetFormData(map[string]string{
				"familyId": y.FamilyID,
			})
		}
	}, &resp)
	if err != nil {
		return nil, err
	}
	if err = y.WaitBatchTask("MOVE", resp.TaskID, time.Millisecond*400); err != nil {
		return nil, err
	}
	return srcObj, nil
}

func (y *Cloud189PC) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
	queryParam := make(map[string]string)
	fullUrl := API_URL
	method := http.MethodPost
	if y.isFamily() {
		fullUrl += "/family/file"
		method = http.MethodGet
		queryParam["familyId"] = y.FamilyID
	}

	var newObj model.Obj
	switch f := srcObj.(type) {
	case *Cloud189File:
		fullUrl += "/renameFile.action"
		queryParam["fileId"] = srcObj.GetID()
		queryParam["destFileName"] = newName
		newObj = &Cloud189File{Icon: f.Icon} // 复用预览
	case *Cloud189Folder:
		fullUrl += "/renameFolder.action"
		queryParam["folderId"] = srcObj.GetID()
		queryParam["destFolderName"] = newName
		newObj = &Cloud189Folder{}
	default:
		return nil, errs.NotSupport
	}

	_, err := y.request(fullUrl, method, func(req *resty.Request) {
		req.SetContext(ctx).SetQueryParams(queryParam)
	}, nil, newObj)
	if err != nil {
		return nil, err
	}
	return newObj, nil
}

func (y *Cloud189PC) Copy(ctx context.Context, srcObj, dstDir model.Obj) error {
	var resp CreateBatchTaskResp
	_, err := y.post(API_URL+"/batch/createBatchTask.action", func(req *resty.Request) {
		req.SetContext(ctx)
		req.SetFormData(map[string]string{
			"type": "COPY",
			"taskInfos": MustString(utils.Json.MarshalToString(
				[]BatchTaskInfo{
					{
						FileId:   srcObj.GetID(),
						FileName: srcObj.GetName(),
						IsFolder: BoolToNumber(srcObj.IsDir()),
					},
				})),
			"targetFolderId": dstDir.GetID(),
			"targetFileName": dstDir.GetName(),
		})
		if y.isFamily() {
			req.SetFormData(map[string]string{
				"familyId": y.FamilyID,
			})
		}
	}, &resp)
	if err != nil {
		return err
	}
	return y.WaitBatchTask("COPY", resp.TaskID, time.Second)
}

func (y *Cloud189PC) Remove(ctx context.Context, obj model.Obj) error {
	var resp CreateBatchTaskResp
	_, err := y.post(API_URL+"/batch/createBatchTask.action", func(req *resty.Request) {
		req.SetContext(ctx)
		req.SetFormData(map[string]string{
			"type": "DELETE",
			"taskInfos": MustString(utils.Json.MarshalToString(
				[]*BatchTaskInfo{
					{
						FileId:   obj.GetID(),
						FileName: obj.GetName(),
						IsFolder: BoolToNumber(obj.IsDir()),
					},
				})),
		})

		if y.isFamily() {
			req.SetFormData(map[string]string{
				"familyId": y.FamilyID,
			})
		}
	}, &resp)
	if err != nil {
		return err
	}
	// 批量任务数量限制，过快会导致无法删除
	return y.WaitBatchTask("DELETE", resp.TaskID, time.Millisecond*200)
}

func (y *Cloud189PC) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	// 响应时间长,按需启用
	if y.Addition.RapidUpload {
		if newObj, err := y.RapidUpload(ctx, dstDir, stream); err == nil {
			return newObj, nil
		}
	}

	switch y.UploadMethod {
	case "old":
		return y.OldUpload(ctx, dstDir, stream, up)
	case "rapid":
		return y.FastUpload(ctx, dstDir, stream, up)
	case "stream":
		if stream.GetSize() == 0 {
			return y.FastUpload(ctx, dstDir, stream, up)
		}
		fallthrough
	default:
		return y.StreamUpload(ctx, dstDir, stream, up)
	}
}

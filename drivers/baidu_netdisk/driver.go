package baidu_netdisk

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	stdpath "path"
	"strconv"
	"strings"

	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	log "github.com/sirupsen/logrus"
)

type BaiduNetdisk struct {
	model.Storage
	Addition
}

func (d *BaiduNetdisk) Config() driver.Config {
	return config
}

func (d *BaiduNetdisk) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *BaiduNetdisk) Init(ctx context.Context) error {
	res, err := d.get("/xpan/nas", map[string]string{
		"method": "uinfo",
	}, nil)
	log.Debugf("[baidu] get uinfo: %s", string(res))
	return err
}

func (d *BaiduNetdisk) Drop(ctx context.Context) error {
	return nil
}

func (d *BaiduNetdisk) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	files, err := d.getFiles(dir.GetPath())
	if err != nil {
		return nil, err
	}
	return utils.SliceConvert(files, func(src File) (model.Obj, error) {
		return fileToObj(src), nil
	})
}

func (d *BaiduNetdisk) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	if d.DownloadAPI == "crack" {
		return d.linkCrack(file, args)
	}
	return d.linkOfficial(file, args)
}

func (d *BaiduNetdisk) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) error {
	_, err := d.create(stdpath.Join(parentDir.GetPath(), dirName), 0, 1, "", "")
	return err
}

func (d *BaiduNetdisk) Move(ctx context.Context, srcObj, dstDir model.Obj) error {
	data := []base.Json{
		{
			"path":    srcObj.GetPath(),
			"dest":    dstDir.GetPath(),
			"newname": srcObj.GetName(),
		},
	}
	_, err := d.manage("move", data)
	return err
}

func (d *BaiduNetdisk) Rename(ctx context.Context, srcObj model.Obj, newName string) error {
	data := []base.Json{
		{
			"path":    srcObj.GetPath(),
			"newname": newName,
		},
	}
	_, err := d.manage("rename", data)
	return err
}

func (d *BaiduNetdisk) Copy(ctx context.Context, srcObj, dstDir model.Obj) error {
	data := []base.Json{
		{
			"path":    srcObj.GetPath(),
			"dest":    dstDir.GetPath(),
			"newname": srcObj.GetName(),
		},
	}
	_, err := d.manage("copy", data)
	return err
}

func (d *BaiduNetdisk) Remove(ctx context.Context, obj model.Obj) error {
	data := []string{obj.GetPath()}
	_, err := d.manage("delete", data)
	return err
}

func (d *BaiduNetdisk) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) error {
	tempFile, err := utils.CreateTempFile(stream.GetReadCloser())
	if err != nil {
		return err
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()
	var Default int64 = 4 * 1024 * 1024
	count := int(math.Ceil(float64(stream.GetSize()) / float64(Default)))
	var SliceSize int64 = 256 * 1024
	// cal md5
	h1 := md5.New()
	h2 := md5.New()
	block_list := make([]string, 0)
	content_md5 := ""
	slice_md5 := ""
	left := stream.GetSize()
	for i := 0; i < count; i++ {
		byteSize := Default
		if left < Default {
			byteSize = left
		}
		left -= byteSize
		_, err = io.Copy(io.MultiWriter(h1, h2), io.LimitReader(tempFile, byteSize))
		if err != nil {
			return err
		}
		block_list = append(block_list, fmt.Sprintf("\"%s\"", hex.EncodeToString(h2.Sum(nil))))
		h2.Reset()
	}
	content_md5 = hex.EncodeToString(h1.Sum(nil))
	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	if stream.GetSize() <= SliceSize {
		slice_md5 = content_md5
	} else {
		sliceData := make([]byte, SliceSize)
		_, err = io.ReadFull(tempFile, sliceData)
		if err != nil {
			return err
		}
		h2.Write(sliceData)
		slice_md5 = hex.EncodeToString(h2.Sum(nil))
		_, err = tempFile.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
	}
	rawPath := stdpath.Join(dstDir.GetPath(), stream.GetName())
	path := encodeURIComponent(rawPath)
	block_list_str := fmt.Sprintf("[%s]", strings.Join(block_list, ","))
	data := fmt.Sprintf("path=%s&size=%d&isdir=0&autoinit=1&block_list=%s&content-md5=%s&slice-md5=%s",
		path, stream.GetSize(),
		block_list_str,
		content_md5, slice_md5)
	params := map[string]string{
		"method": "precreate",
	}
	log.Debugf("[baidu_netdisk] precreate data: %s", data)
	var precreateResp PrecreateResp
	_, err = d.post("/xpan/file", params, data, &precreateResp)
	if err != nil {
		return err
	}
	log.Debugf("%+v", precreateResp)
	if precreateResp.ReturnType == 2 {
		return nil
	}
	params = map[string]string{
		"method":       "upload",
		"access_token": d.AccessToken,
		"type":         "tmpfile",
		"path":         path,
		"uploadid":     precreateResp.Uploadid,
	}
	left = stream.GetSize()
	for i, partseq := range precreateResp.BlockList {
		if utils.IsCanceled(ctx) {
			return ctx.Err()
		}
		byteSize := Default
		if left < Default {
			byteSize = left
		}
		left -= byteSize
		u := "https://d.pcs.baidu.com/rest/2.0/pcs/superfile2"
		params["partseq"] = strconv.Itoa(partseq)
		res, err := base.RestyClient.R().
			SetContext(ctx).
			SetQueryParams(params).
			SetFileReader("file", stream.GetName(), io.LimitReader(tempFile, byteSize)).
			Post(u)
		if err != nil {
			return err
		}
		log.Debugln(res.String())
		if len(precreateResp.BlockList) > 0 {
			up(i * 100 / len(precreateResp.BlockList))
		}
	}
	_, err = d.create(rawPath, stream.GetSize(), 0, precreateResp.Uploadid, block_list_str)
	return err
}

var _ driver.Driver = (*BaiduNetdisk)(nil)

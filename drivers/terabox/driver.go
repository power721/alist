package terbox

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/pkg/utils"
	log "github.com/sirupsen/logrus"
	"io"
	"math"
	"os"
	stdpath "path"
	"strconv"
	"strings"

	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/model"
)

type Terabox struct {
	model.Storage
	Addition
}

func (d *Terabox) Config() driver.Config {
	return config
}

func (d *Terabox) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *Terabox) Init(ctx context.Context) error {
	var resp CheckLoginResp
	_, err := d.get("/api/check/login", nil, &resp)
	if err != nil {
		return err
	}
	if resp.Errno != 0 {
		if resp.Errno == 9000 {
			return fmt.Errorf("terabox is not yet available in this area")
		}
		return fmt.Errorf("failed to check login status according to cookie")
	}
	return err
}

func (d *Terabox) Drop(ctx context.Context) error {
	return nil
}

func (d *Terabox) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	files, err := d.getFiles(dir.GetPath())
	if err != nil {
		return nil, err
	}
	return utils.SliceConvert(files, func(src File) (model.Obj, error) {
		return fileToObj(src), nil
	})
}

func (d *Terabox) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	if d.DownloadAPI == "crack" {
		return d.linkCrack(file, args)
	}
	return d.linkOfficial(file, args)
}

func (d *Terabox) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) error {
	_, err := d.create(stdpath.Join(parentDir.GetPath(), dirName), 0, 1, "", "")
	return err
}

func (d *Terabox) Move(ctx context.Context, srcObj, dstDir model.Obj) error {
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

func (d *Terabox) Rename(ctx context.Context, srcObj model.Obj, newName string) error {
	data := []base.Json{
		{
			"path":    srcObj.GetPath(),
			"newname": newName,
		},
	}
	_, err := d.manage("rename", data)
	return err
}

func (d *Terabox) Copy(ctx context.Context, srcObj, dstDir model.Obj) error {
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

func (d *Terabox) Remove(ctx context.Context, obj model.Obj) error {
	data := []string{obj.GetPath()}
	_, err := d.manage("delete", data)
	return err
}

func (d *Terabox) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) error {
	tempFile, err := utils.CreateTempFile(stream.GetReadCloser())
	if err != nil {
		return err
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()
	var Default int64 = 4 * 1024 * 1024
	defaultByteData := make([]byte, Default)
	count := int(math.Ceil(float64(stream.GetSize()) / float64(Default)))
	// cal md5
	h1 := md5.New()
	h2 := md5.New()
	block_list := make([]string, 0)
	left := stream.GetSize()
	for i := 0; i < count; i++ {
		byteSize := Default
		var byteData []byte
		if left < Default {
			byteSize = left
			byteData = make([]byte, byteSize)
		} else {
			byteData = defaultByteData
		}
		left -= byteSize
		_, err = io.ReadFull(tempFile, byteData)
		if err != nil {
			return err
		}
		h1.Write(byteData)
		h2.Write(byteData)
		block_list = append(block_list, fmt.Sprintf("\"%s\"", hex.EncodeToString(h2.Sum(nil))))
		h2.Reset()
	}

	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	rawPath := stdpath.Join(dstDir.GetPath(), stream.GetName())
	path := encodeURIComponent(rawPath)
	block_list_str := fmt.Sprintf("[%s]", strings.Join(block_list, ","))
	data := fmt.Sprintf("path=%s&size=%d&isdir=0&autoinit=1&block_list=%s",
		path, stream.GetSize(),
		block_list_str)
	params := map[string]string{}
	var precreateResp PrecreateResp
	_, err = d.post("/api/precreate", params, data, &precreateResp)
	if err != nil {
		return err
	}
	log.Debugf("%+v", precreateResp)
	if precreateResp.ReturnType == 2 {
		return nil
	}
	params = map[string]string{
		"method":     "upload",
		"path":       path,
		"uploadid":   precreateResp.Uploadid,
		"app_id":     "250528",
		"web":        "1",
		"channel":    "dubox",
		"clienttype": "0",
	}
	left = stream.GetSize()
	for i, partseq := range precreateResp.BlockList {
		if utils.IsCanceled(ctx) {
			return ctx.Err()
		}
		byteSize := Default
		var byteData []byte
		if left < Default {
			byteSize = left
			byteData = make([]byte, byteSize)
		} else {
			byteData = defaultByteData
		}
		left -= byteSize
		_, err = io.ReadFull(tempFile, byteData)
		if err != nil {
			return err
		}
		u := "https://c-jp.terabox.com/rest/2.0/pcs/superfile2"
		params["partseq"] = strconv.Itoa(partseq)
		res, err := base.RestyClient.R().
			SetContext(ctx).
			SetQueryParams(params).
			SetFileReader("file", stream.GetName(), bytes.NewReader(byteData)).
			SetHeader("Cookie", d.Cookie).
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

var _ driver.Driver = (*Terabox)(nil)

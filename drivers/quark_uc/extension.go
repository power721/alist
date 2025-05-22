package quark

import (
	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
	"net/http"
)

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

func (d *QuarkOrUC) deleteFile(fileId string) error {
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
		log.Warnf("Delete %v temp file failed: %v %v", d.Config().Name, fileId, err)
		return err
	}
	return nil
}

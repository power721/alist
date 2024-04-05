package aliyundrive_share2_open

import (
	"context"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/setting"
	"github.com/alist-org/alist/v3/pkg/utils"
	log "github.com/sirupsen/logrus"
)

type QuarkShare struct {
	model.Storage
	Addition
}

func (d *QuarkShare) Config() driver.Config {
	return config
}

func (d *QuarkShare) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *QuarkShare) Init(ctx context.Context) error {
	if !initialized {
		Cookie = setting.GetStr("quark_cookie", "")
		d.getTempFolder()
		log.Infof("ParentFileId: %v", ParentFileId)
		d.cleanTempFolder()
		initialized = true
	}

	err := d.getShareToken()
	if err != nil {
		log.Errorf("getShareToken error: %v", err)
		return err
	}

	return nil
}

func (d *QuarkShare) Drop(ctx context.Context) error {
	return nil
}

func (d *QuarkShare) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	files, err := d.getFiles(dir.GetID())
	if err != nil {
		log.Warnf("list files error: %v", err)
		return nil, err
	}
	return utils.SliceConvert(files, func(src File) (model.Obj, error) {
		return fileToObj(src), nil
	})
}

func (d *QuarkShare) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	log.Infof("获取文件直链 %v %v %v", file.GetName(), file.GetID(), file.GetSize())
	fileId, err := d.saveFile(file.GetID())
	if err != nil {
		return nil, err
	}

	return d.getDownloadUrl(fileId)
}

var _ driver.Driver = (*QuarkShare)(nil)

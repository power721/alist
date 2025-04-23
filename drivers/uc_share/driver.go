package uc_share

import (
	"context"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/token"
	"github.com/alist-org/alist/v3/pkg/utils"
	log "github.com/sirupsen/logrus"
)

type UcShare struct {
	model.Storage
	Addition
}

func (d *UcShare) Config() driver.Config {
	return config
}

func (d *UcShare) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *UcShare) Init(ctx context.Context) error {
	if Cookie == "" {
		Cookie = token.GetAccountToken(conf.UC)
		d.getTempFolder()
		log.Infof("UcShare ParentFileId: %v", ParentFileId)
		d.cleanTempFolder()
	}

	err := d.getShareToken()
	if err != nil {
		log.Errorf("getShareToken error: %v", err)
		return err
	}

	return nil
}

func (d *UcShare) Drop(ctx context.Context) error {
	return nil
}

func (d *UcShare) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	files, err := d.getShareFiles(dir.GetID())
	if err != nil {
		log.Warnf("list files error: %v", err)
		return nil, err
	}
	return utils.SliceConvert(files, func(src File) (model.Obj, error) {
		return fileToObj(src), nil
	})
}

func (d *UcShare) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	log.Infof("获取文件直链 %v %v %v", file.GetName(), file.GetID(), file.GetSize())
	fileId, err := d.saveFile(file.GetID())
	if err != nil {
		return nil, err
	}

	return d.getDownloadUrl(ctx, MyFile{FileId: fileId}, args)
}

var _ driver.Driver = (*UcShare)(nil)

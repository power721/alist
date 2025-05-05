package _139_share

import (
	"context"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	log "github.com/sirupsen/logrus"
	"time"
)

type Yun139Share struct {
	model.Storage
	Addition
}

func (d *Yun139Share) Config() driver.Config {
	return config
}

func (d *Yun139Share) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *Yun139Share) Init(ctx context.Context) error {
	return nil
}

func (d *Yun139Share) Drop(ctx context.Context) error {
	return nil
}

func (d *Yun139Share) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	files, err := d.list(dir.GetID())
	if err != nil {
		log.Warnf("list files error: %v", err)
		return nil, err
	}
	return utils.SliceConvert(files, func(src File) (model.Obj, error) {
		return fileToObj(src), nil
	})
}

func (d *Yun139Share) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	log.Debugf("获取文件直链 %v %v %v", file.GetName(), file.GetID(), file.GetSize())
	url, err := d.link(file.GetID())
	if err != nil {
		return nil, err
	}
	exp := 895 * time.Second
	return &model.Link{URL: url, Expiration: &exp}, nil
}

var _ driver.Driver = (*Yun139Share)(nil)

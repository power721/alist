package thunder_share

import (
	"context"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/model"
	log "github.com/sirupsen/logrus"
)

type ThunderShare struct {
	model.Storage
	Addition
}

func (d *ThunderShare) Config() driver.Config {
	return config
}

func (d *ThunderShare) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *ThunderShare) Init(ctx context.Context) error {
	return nil
}

func (d *ThunderShare) Drop(ctx context.Context) error {
	return nil
}

func (d *ThunderShare) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	files, err := d.listShareFiles(ctx, dir)
	if err != nil {
		log.Warnf("list files error: %v", err)
		return nil, err
	}
	return files, err
}

func (d *ThunderShare) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	log.Infof("获取文件直链 %v %v %v", file.GetName(), file.GetID(), file.GetSize())
	fileId, err := d.saveFile(ctx, file)
	if err != nil {
		log.Warnf("保存文件失败: %v", err)
		return nil, err
	}

	return d.getDownloadUrl(ctx, fileId)
}

var _ driver.Driver = (*ThunderShare)(nil)

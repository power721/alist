package _115_share

import (
	"context"

	driver115 "github.com/SheltonZhu/115driver/pkg/driver"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	"golang.org/x/time/rate"
)

type Pan115Share struct {
	model.Storage
	Addition
	limiter *rate.Limiter
}

func (d *Pan115Share) Config() driver.Config {
	return config
}

func (d *Pan115Share) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *Pan115Share) Init(ctx context.Context) error {
	if d.LimitRate > 0 {
		d.limiter = rate.NewLimiter(rate.Limit(d.LimitRate), 1)
	}

	if !initialized {
		err := d.login()
		if err != nil {
			return err
		}
		initialized = true
	}

	return nil
}

func (d *Pan115Share) WaitLimit(ctx context.Context) error {
	if d.limiter != nil {
		return d.limiter.Wait(ctx)
	}
	return nil
}

func (d *Pan115Share) Drop(ctx context.Context) error {
	return nil
}

func (d *Pan115Share) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	if err := d.WaitLimit(ctx); err != nil {
		return nil, err
	}

	files := make([]driver115.ShareFile, 0)
	fileResp, err := client.GetShareSnap(d.ShareCode, d.ReceiveCode, dir.GetID(), driver115.QueryLimit(int(d.PageSize)))
	if err != nil {
		return nil, err
	}
	files = append(files, fileResp.Data.List...)
	total := fileResp.Data.Count
	count := len(fileResp.Data.List)
	for total > count {
		fileResp, err := client.GetShareSnap(
			d.ShareCode, d.ReceiveCode, dir.GetID(),
			driver115.QueryLimit(int(d.PageSize)), driver115.QueryOffset(count),
		)
		if err != nil {
			return nil, err
		}
		files = append(files, fileResp.Data.List...)
		count += len(fileResp.Data.List)
	}

	return utils.SliceConvert(files, transFunc)
}

func (d *Pan115Share) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	if err := d.WaitLimit(ctx); err != nil {
		return nil, err
	}
	downloadInfo, err := client.DownloadByShareCode(d.ShareCode, d.ReceiveCode, file.GetID())
	if err != nil {
		return nil, err
	}

	return &model.Link{URL: downloadInfo.URL.URL}, nil
}

var _ driver.Driver = (*Pan115Share)(nil)

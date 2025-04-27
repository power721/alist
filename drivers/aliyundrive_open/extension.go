package aliyundrive_open

import (
	"context"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/token"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

func (d *AliyundriveOpen) SaveOpenToken(t time.Time) {
	accountId := strconv.Itoa(d.AccountId)
	item := &model.Token{
		Key:       "AccessTokenOpen-" + accountId,
		Value:     d.AccessToken,
		AccountId: d.AccountId,
		Modified:  t,
	}

	err := token.SaveToken(item)
	if err != nil {
		log.Warnf("save AccessTokenOpen failed: %v", err)
	}

	item = &model.Token{
		Key:       "RefreshTokenOpen-" + accountId,
		Value:     d.RefreshToken,
		AccountId: d.AccountId,
		Modified:  t,
	}

	err = token.SaveToken(item)
	if err != nil {
		log.Warnf("save RefreshTokenOpen failed: %v", err)
	}
}

func (d *AliyundriveOpen) createTempDir(ctx context.Context) {
	dir := &model.Object{
		ID:   d.TempDirId,
		Path: "",
	}

	res, err := d.MakeDir(ctx, dir, conf.TempDirName)

	if err != nil {
		log.Warnf("创建阿里缓存文件夹失败: %v", err)
	} else {
		d.TempDirId = res.GetID()
	}

	if d.TempDirId == "" {
		d.TempDirId = "root"
	}
	log.Printf("阿里缓存文件夹ID： %v", d.TempDirId)
}

func (d *AliyundriveOpen) cleanTempFolder(ctx context.Context) {
	if d.TempDirId == "root" {
		return
	}

	dir := &model.Object{
		ID:   d.TempDirId,
		Path: "",
	}

	files, err := d.List(ctx, dir, model.ListArgs{})
	if err != nil {
		log.Errorf("获取文件列表失败 %v", err)
		return
	}

	for _, file := range files {
		log.Infof("删除文件 %v %v", file.GetName(), file.GetID())
		f := &model.Object{
			ID: file.GetID(),
		}
		_ = d.Remove(ctx, f)
	}
}

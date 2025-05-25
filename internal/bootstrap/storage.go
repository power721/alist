package bootstrap

import (
	"context"
	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/setting"

	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/db"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	log "github.com/sirupsen/logrus"
)

func LoadStorages() {
	storages, err := db.GetEnabledStorages()
	if err != nil {
		log.Fatalf("failed get enabled storages: %+v", err)
	}
	log.Infof("total %v enabled storages", len(storages))

	go func(storages []model.Storage) {
		for i := range storages {
			err := op.LoadStorage(context.Background(), storages[i])
			if err != nil {

				log.Errorf("[%d] failed get enabled storages [%s], %+v",
					i+1, storages[i].MountPath, err)
			} else {
				log.Infof("[%d] success load storage: [%s], driver: [%s]",
					i+1, storages[i].MountPath, storages[i].Driver)
			}
		}
		log.Infof("=== load storages completed ===")
		syncStatus()
		conf.StoragesLoaded = true
	}(storages)
}

func syncStatus() {
	url := "http://127.0.0.1:4567/api/alist/status?code=2"
	_, err := base.RestyClient.R().
		SetHeader("X-API-KEY", setting.GetStr("atv_api_key")).
		Post(url)
	if err != nil {
		log.Warnf("sync status failed: %v", err)
	}
}

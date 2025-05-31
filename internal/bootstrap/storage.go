package bootstrap

import (
	"context"
	_189_share "github.com/alist-org/alist/v3/drivers/189_share"
	"github.com/alist-org/alist/v3/drivers/aliyundrive_share2_open"
	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/setting"
	"time"

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
			storage := storages[i]
			err := op.LoadStorage(context.Background(), storage)
			if err != nil {
				log.Errorf("[%d] failed get enabled storages [%s], %+v",
					i+1, storage.MountPath, err)
			} else {
				log.Infof("[%d] success load storage: [%s], driver: [%s]",
					i+1, storage.MountPath, storage.Driver)
			}
		}
		log.Infof("=== load storages completed ===")
		syncStatus()
		validate()
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

func validate() {
	go validateAliShares()
	go validate189Shares()
}

func validateAliShares() {
	storages := op.GetStorages("AliyundriveShare2Open")
	log.Infof("validate ali shares")
	for _, storage := range storages {
		ali := storage.(*aliyundrive_share2_open.AliyundriveShare2Open)
		if ali.ID < 20000 {
			continue
		}
		err := ali.GetShareToken()
		if err != nil {
			log.Warnf("[%v] failed get share token: %v", ali.ID, err)
			ali.GetStorage().SetStatus(err.Error())
			op.MustSaveDriverStorage(ali)
		}
		time.Sleep(1500 * time.Millisecond)
	}
}

func validate189Shares() {
	storages := op.GetStorages("189Share")
	log.Infof("validate 189 shares")
	for _, storage := range storages {
		driver := storage.(*_189_share.Cloud189Share)
		if driver.ID < 20000 {
			continue
		}
		_, err := driver.GetShareInfo()
		if err != nil {
			log.Warnf("[%v] failed get share info: %v", driver.ID, err)
			driver.GetStorage().SetStatus(err.Error())
			op.MustSaveDriverStorage(driver)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

package bootstrap

import (
	"context"
	"github.com/alist-org/alist/v3/drivers/aliyundrive_share2_open"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/db"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/pkg/utils"
	log "github.com/sirupsen/logrus"
)

func LoadStorages() {
	storages, err := db.GetEnabledStorages()
	if err != nil {
		utils.Log.Fatalf("failed get enabled storages: %+v", err)
	}
	log.Infof("total %v enabled storages", len(storages))

	go func(storages []model.Storage) {
		var failed []model.Storage
		for i := range storages {
			err := op.LoadStorage(context.Background(), storages[i])
			if err != nil {
				failed = append(failed, storages[i])
				utils.Log.Warnf("[%d] failed get enabled storages [%s], will retry: %+v",
					i+1, storages[i].MountPath, err)
			} else {
				utils.Log.Infof("[%d] success load storage: [%s], driver: [%s]",
					i+1, storages[i].MountPath, storages[i].Driver)
			}
		}

		if len(failed) > 0 {
			aliyundrive_share2_open.DelayTime = 2000
			utils.Log.Infof("retry %v failed storages", len(failed))
			for i := range failed {
				err := op.LoadStorage(context.Background(), failed[i])
				if err != nil {
					utils.Log.Errorf("failed get enabled storages [%s]: %+v", failed[i].MountPath, err)
				} else {
					utils.Log.Infof("success load storage: [%s], driver: [%s]",
						failed[i].MountPath, failed[i].Driver)
				}
			}
		}

		conf.StoragesLoaded = true
	}(storages)
}

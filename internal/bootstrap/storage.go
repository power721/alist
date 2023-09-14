package bootstrap

import (
	"context"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/db"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/pkg/utils"
)

func LoadStorages() {
	storages, err := db.GetEnabledStorages()
	if err != nil {
		utils.Log.Fatalf("failed get enabled storages: %+v", err)
	}
	go func(storages []model.Storage) {
		var failed []model.Storage
		for i := range storages {
			err := op.LoadStorage(context.Background(), storages[i])
			if err != nil {
				failed = append(failed, storages[i])
				utils.Log.Warnf("failed get enabled storages [%s], will retry: %+v", storages[i].MountPath, err)
			} else {
				utils.Log.Infof("success load storage: [%s], driver: [%s]",
					storages[i].MountPath, storages[i].Driver)
			}
		}

		if len(failed) > 0 {
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

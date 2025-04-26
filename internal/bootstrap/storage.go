package bootstrap

import (
	"context"

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
		log.Infof("load storages completed")
		conf.StoragesLoaded = true
	}(storages)
}

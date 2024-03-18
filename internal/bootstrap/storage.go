package bootstrap

import (
	"context"
	"github.com/alist-org/alist/v3/drivers/aliyundrive_share2_open"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/db"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	log "github.com/sirupsen/logrus"
	"strings"
)

func LoadStorages() {
	storages, err := db.GetEnabledStorages()
	if err != nil {
		log.Fatalf("failed get enabled storages: %+v", err)
	}
	log.Infof("total %v enabled storages", len(storages))

	go func(storages []model.Storage) {
		var failed []model.Storage
		for i := range storages {
			err := op.LoadStorage(context.Background(), storages[i])
			if err != nil {
				msg := err.Error()
				if strings.Contains(msg, "share_link is cancelled") ||
					strings.Contains(msg, "share_link is forbidden") ||
					strings.Contains(msg, "share_link is expired") ||
					strings.Contains(msg, "share_link cannot be found") ||
					strings.Contains(msg, "share_pwd is not valid") ||
					strings.Contains(msg, "invalid") ||
					strings.Contains(msg, "no route to host") {
					log.Warnf("[%d] failed get enabled storages [%s], %+v",
						i+1, storages[i].MountPath, err)
				} else {
					failed = append(failed, storages[i])
					log.Warnf("[%d] failed get enabled storages [%s], will retry: %+v",
						i+1, storages[i].MountPath, err)
				}
			} else {
				log.Infof("[%d] success load storage: [%s], driver: [%s]",
					i+1, storages[i].MountPath, storages[i].Driver)
			}
		}

		if len(failed) > 0 {
			aliyundrive_share2_open.DelayTime = 2000
			log.Infof("retry %v failed storages", len(failed))
			for i := range failed {
				err := op.LoadStorage(context.Background(), failed[i])
				if err != nil {
					log.Errorf("failed get enabled storages [%s]: %+v", failed[i].MountPath, err)
				} else {
					log.Infof("success load storage: [%s], driver: [%s]",
						failed[i].MountPath, failed[i].Driver)
				}
			}
		}

		conf.StoragesLoaded = true
	}(storages)
}

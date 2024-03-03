package bootstrap

import (
	"github.com/alist-org/alist/v3/internal/offline_download/tool"
	log "github.com/sirupsen/logrus"
)

func InitOfflineDownloadTools() {
	for k, v := range tool.Tools {
		res, err := v.Init()
		if err != nil {
			log.Warnf("init tool %s failed: %s", k, err)
		} else {
			log.Infof("init tool %s success: %s", k, res)
		}
	}
}

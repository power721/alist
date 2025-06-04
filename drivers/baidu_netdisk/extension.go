package baidu_netdisk

import (
	"github.com/alist-org/alist/v3/internal/conf"
	log "github.com/sirupsen/logrus"
	stdpath "path"
)

func (d *BaiduNetdisk) createTempDir() error {
	var newDir File
	_, err := d.create(stdpath.Join("/", conf.TempDirName), 0, 1, "", "", &newDir, 0, 0)
	if err != nil {
		log.Warnf("create temp dir failed: %v", err)

		files, err := d.getFiles("/")
		if err != nil {
			return err
		}

		for _, file := range files {
			if file.ServerFilename == conf.TempDirName {
				d.TempDirId = file.FsId
				break
			}
		}
	} else {
		d.TempDirId = newDir.FsId
	}

	log.Info("BaiduNetdisk temp folder id: ", d.TempDirId)
	return nil
}

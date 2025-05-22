package thunder_browser

import (
	"context"
	"github.com/alist-org/alist/v3/internal/conf"
	log "github.com/sirupsen/logrus"
)

func (y *ThunderBrowser) createTempDir(ctx context.Context) error {
	dir := &Files{
		ID:    "",
		Space: "",
	}
	err := y.MakeDir(ctx, dir, conf.TempDirName)
	if err != nil {
		log.Warnf("create Thunder temp dir failed: %v", err)
	}

	files, err := y.getFiles(ctx, dir, "")
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.GetName() == conf.TempDirName {
			y.TempDirId = file.GetID()
			break
		}
	}

	log.Info("Thunder temp folder id: ", y.TempDirId)
	return nil
}

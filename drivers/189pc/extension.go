package _189pc

import (
	"context"
	"errors"
	"github.com/alist-org/alist/v3/internal/model"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

const TransferPath = "xiaoya-tvbox-temp"

var tempDirId = "-11"

func (y *Cloud189PC) createTempDir(ctx context.Context) error {
	dir := &Cloud189File{
		ID: "-11",
	}
	_, err := y.MakeDir(ctx, dir, TransferPath)
	if err != nil {
		log.Warnf("create temp dir failed: %v", err)
	}

	files, err := y.getFiles(ctx, tempDirId, false)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.GetName() == TransferPath {
			tempDirId = file.GetID()
			break
		}
	}

	log.Info("189Cloud temp folder id: ", tempDirId)
	return nil
}

func (y *Cloud189PC) Transfer(ctx context.Context, shareId int, fileId string, fileName string) (*model.Link, error) {

	isFamily := y.isFamily()
	other := map[string]string{"shareId": strconv.Itoa(shareId)}

	log.Debug("create share save task")
	resp, err := y.CreateBatchTask("SHARE_SAVE", IF(isFamily, y.FamilyID, ""), tempDirId, other, BatchTaskInfo{
		FileId:   fileId,
		FileName: fileName,
		IsFolder: 0,
	})

	if err != nil && !strings.Contains(err.Error(), "there is a conflict with the target object") {
		return nil, err
	}

	log.Debug("wait task")
	err = y.WaitBatchTask("SHARE_SAVE", resp.TaskID, time.Second)
	if err != nil && !strings.Contains(err.Error(), "there is a conflict with the target object") {
		return nil, err
	}

	log.Debug("get files")
	files, err := y.getFiles(ctx, tempDirId, false)
	if err != nil {
		return nil, err
	}

	log.Debug("get new file")
	var transferFile model.Obj
	for _, file := range files {
		if file.GetName() == fileName {
			transferFile = file
			break
		}
	}

	if transferFile == nil || transferFile.GetID() == "" {
		return nil, errors.New("文件转存失败")
	}

	log.Debug("get new file link")
	link, err := y.Link(ctx, transferFile, model.LinkArgs{})

	go func() {
		removeErr := y.Remove(ctx, transferFile)
		if removeErr != nil {
			log.Infof("天翼云盘删除文件:%s失败:%v", fileName, removeErr)
			return
		}
		log.Debugf("已删除天翼云盘下的文件:%s", fileName)
		_, removeErr = y.CreateBatchTask("CLEAR_RECYCLE", "", "", nil, BatchTaskInfo{
			FileId:   transferFile.GetID(),
			FileName: transferFile.GetName(),
			IsFolder: 0,
		})
		if removeErr != nil {
			log.Info("天翼云盘清除回收站失败", removeErr)
		} else {
			log.Debug("天翼云盘清除回收站完成")
		}
	}()

	return link, err
}

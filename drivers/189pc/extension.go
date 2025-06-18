package _189pc

import (
	"context"
	"errors"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/setting"
	"github.com/alist-org/alist/v3/pkg/cron"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

func (y *Cloud189PC) createTempDir(ctx context.Context) error {
	dir := &Cloud189File{
		ID: "-11",
	}
	_, err := y.MakeDir(ctx, dir, conf.TempDirName)
	if err != nil {
		log.Warnf("create temp dir failed: %v", err)
	}

	files, err := y.getFiles(ctx, y.TempDirId, false)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.GetName() == conf.TempDirName {
			y.TempDirId = file.GetID()
			break
		}
	}

	log.Info("189Cloud temp folder id: ", y.TempDirId)
	return nil
}

func (y *Cloud189PC) Checkin() {
	y.checkin()
	y.cron = cron.NewCron(time.Hour * 24)
	y.cron.Do(func() {
		y.checkin()
	})
}

func (y *Cloud189PC) checkin() {
	url := API_URL + "/mkt/userSign.action"
	res, err := y.get(url, nil, nil)
	log.Infof("checkin result: %s", string(res))
	if err != nil {
		log.Warnf("checkin failed: %v", err)
	}

	res, err = y.get("https://m.cloud.189.cn/v2/drawPrizeMarketDetails.action?taskId=TASK_SIGNIN&activityId=ACT_SIGNIN", nil, nil)
	log.Infof("TASK_SIGNIN result: %s", string(res))
	if err != nil {
		log.Warnf("TASK_SIGNIN failed: %v", err)
	}

	//res, err = y.get("https://m.cloud.189.cn/v2/drawPrizeMarketDetails.action?taskId=TASK_SIGNIN_PHOTOS&activityId=ACT_SIGNIN", nil, nil)
	//log.Infof("TASK_SIGNIN_PHOTOS result: %s", string(res))
	//if err != nil {
	//	log.Warnf("TASK_SIGNIN_PHOTOS failed: %v", err)
	//}
}

func (y *Cloud189PC) Transfer(ctx context.Context, shareId int, fileId string, fileName string) (*model.Link, error) {

	isFamily := y.isFamily()
	other := map[string]string{"shareId": strconv.Itoa(shareId)}

	log.Debug("create share save task")
	resp, err := y.CreateBatchTask("SHARE_SAVE", IF(isFamily, y.FamilyID, ""), y.TempDirId, other, BatchTaskInfo{
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
	files, err := y.getFiles(ctx, y.TempDirId, false)
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
		delayTime := setting.GetInt(conf.DeleteDelayTime, 900)
		if delayTime == 0 {
			return
		}

		log.Infof("[%v] Delete 189 temp file %v after %v seconds.", y.ID, fileId, delayTime)
		time.Sleep(time.Duration(delayTime) * time.Second)

		log.Infof("[%v] Delete 189 temp file: %v %v", y.ID, fileId, fileName)
		removeErr := y.Remove(ctx, transferFile)
		if removeErr != nil {
			log.Infof("[%v] 天翼云盘删除文件:%s失败: %v", y.ID, fileName, removeErr)
			return
		}
		log.Debugf("[%v] 已删除天翼云盘下的文件: %v", y.ID, fileName)
		_, removeErr = y.CreateBatchTask("CLEAR_RECYCLE", "", "", nil, BatchTaskInfo{
			FileId:   transferFile.GetID(),
			FileName: transferFile.GetName(),
			IsFolder: 0,
		})
		if removeErr != nil {
			log.Infof("[%v] 天翼云盘清除回收站失败: %v", y.ID, removeErr)
		} else {
			log.Debugf("[%v] 天翼云盘清除回收站完成", y.ID)
		}
	}()

	return link, err
}

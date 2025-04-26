package aliyundrive_open

import (
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/token"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

func (d *AliyundriveOpen) SaveOpenToken(t time.Time) {
	accountId := strconv.Itoa(d.AccountId)
	item := &model.Token{
		Key:       "AccessTokenOpen-" + accountId,
		Value:     d.AccessToken,
		AccountId: d.AccountId,
		Modified:  t,
	}

	err := token.SaveToken(item)
	if err != nil {
		log.Warnf("save AccessTokenOpen failed: %v", err)
	}

	item = &model.Token{
		Key:       "RefreshTokenOpen-" + accountId,
		Value:     d.RefreshToken,
		AccountId: d.AccountId,
		Modified:  t,
	}

	err = token.SaveToken(item)
	if err != nil {
		log.Warnf("save RefreshTokenOpen failed: %v", err)
	}
}

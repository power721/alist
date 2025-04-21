package token

import (
	"strconv"
	"time"

	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/internal/setting"
	"github.com/alist-org/alist/v3/pkg/utils"
	log "github.com/sirupsen/logrus"
)

func GetToken(key string, expire float64, defaultValue ...string) string {
	val, _ := op.GetTokenByKey(key)
	if val == nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return ""
	}
	if expire > 0 {
		diff := time.Now().Sub(val.Modified)
		utils.Log.Debugf("%v %v %v", key, val, diff)
		if diff.Seconds() >= expire {
			utils.Log.Printf("%v expired at %v", key, val.Modified)
			return ""
		}
	}

	return val.Value
}

func SaveToken(item *model.Token) (err error) {
	return op.SaveToken(item)
}

func SaveAccountToken(prefix, value string, accountId int) {
	key := prefix + "_" + strconv.Itoa(accountId)
	item := &model.Token{
		Key:       key,
		Value:     value,
		AccountId: accountId,
		Modified:  time.Now(),
	}
	log.Infof("save account token: %v", key)
	err := SaveToken(item)
	if err != nil {
		log.Warnf("save account token failed: %v %v", key, err)
	}
}

func GetAccountToken(prefix string) string {
	accountId := setting.GetInt(prefix+"_id", 1)
	key := prefix + "_" + strconv.Itoa(accountId)
	return GetToken(key, 0)
}

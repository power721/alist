package token

import (
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/pkg/utils"
	"time"
)

func GetToken(key string, defaultValue ...string) string {
	val, _ := op.GetTokenByKey(key)
	if val == nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return ""
	}
	diff := time.Now().Sub(val.Modified)
	utils.Log.Debugf("%v %v %v", key, val, diff)
	if diff.Seconds() >= 7200 {
		return ""
	}
	return val.Value
}

func SaveToken(item *model.Token) (err error) {
	return op.SaveToken(item)
}

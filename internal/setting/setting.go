package setting

import (
	"github.com/alist-org/alist/v3/internal/model"
	"strconv"

	"github.com/alist-org/alist/v3/internal/op"
)

func GetStr(key string, defaultValue ...string) string {
	val, _ := op.GetSettingItemByKey(key)
	if val == nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return ""
	}
	return val.Value
}

func GetInt(key string, defaultVal int) int {
	i, err := strconv.Atoi(GetStr(key))
	if err != nil {
		return defaultVal
	}
	return i
}

func GetInt64(key string, defaultVal int64) int64 {
	i, err := strconv.ParseInt(GetStr(key), 10, 0)
	if err != nil {
		return defaultVal
	}
	return i
}

func GetBool(key string) bool {
	return GetStr(key) == "true" || GetStr(key) == "1"
}

func SaveSetting(item *model.SettingItem) (err error) {
	return op.SaveSettingItem(item)
}

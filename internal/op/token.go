package op

import (
	"time"

	"github.com/Xhofe/go-cache"
	"github.com/alist-org/alist/v3/internal/db"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/singleflight"
	"github.com/pkg/errors"
)

var tokenCache = cache.NewMemCache(cache.WithShards[*model.Token](4))
var tokenG singleflight.Group[*model.Token]
var tokenCacheF = func(item *model.Token) {
	tokenCache.Set(item.Key, item, cache.WithEx[*model.Token](time.Hour))
}

func tokenCacheUpdate() {
	tokenCache.Clear()
}

func GetTokens() ([]model.Token, error) {
	items, err := db.GetTokens()
	if err != nil {
		return nil, err
	}
	return items, err
}

func GetTokenByKey(key string) (*model.Token, error) {
	if item, ok := tokenCache.Get(key); ok {
		return item, nil
	}

	item, err, _ := tokenG.Do(key, func() (*model.Token, error) {
		_item, err := db.GetTokenByKey(key)
		if err != nil {
			return nil, err
		}
		tokenCacheF(_item)
		return _item, nil
	})
	return item, err
}

func SaveToken(item *model.Token) (err error) {
	// update
	if err = db.SaveToken(item); err != nil {
		return err
	}
	tokenCacheUpdate()
	return nil
}

func DeleteTokenByKey(key string) error {
	_, err := GetTokenByKey(key)
	if err != nil {
		return errors.WithMessage(err, "failed to get token")
	}
	tokenCacheUpdate()
	return db.DeleteTokenByKey(key)
}

package _115_share

import (
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/op"
)

type Addition struct {
	PageSize    int64   `json:"page_size" type:"number" default:"1000" help:"list api per page size of 115 driver"`
	LimitRate   float64 `json:"limit_rate" type:"number" default:"2" help:"limit all api request rate (1r/[limit_rate]s)"`
	ShareCode   string  `json:"share_code" type:"text" required:"true" help:"share code of 115 share link"`
	ReceiveCode string  `json:"receive_code" type:"text" required:"true" help:"receive code of 115 share link"`
	driver.RootID
}

var config = driver.Config{
	Name:        "115 Share",
	DefaultRoot: "0",
	// OnlyProxy:   true,
	// OnlyLocal:         true,
	CheckStatus:       false,
	Alert:             "",
	NoOverwriteUpload: true,
	NoUpload:          true,
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &Pan115Share{}
	})
}

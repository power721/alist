package _115_share

import (
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/op"
)

type Addition struct {
	ShareCode   string `json:"share_code" type:"text" required:"true" help:"share code of 115 share link"`
	ReceiveCode string `json:"receive_code" type:"text" required:"true" help:"receive code of 115 share link"`
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

package cloudreve

import (
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/op"
)

type Addition struct {
	// Usually one of two
	driver.RootPath
	// define other
	Address  string `json:"address" required:"true"`
	Username string `json:"username"`
	Password string `json:"password"`
	Cookie   string `json:"cookie"`
}

var config = driver.Config{
	Name:        "Cloudreve",
	DefaultRoot: "/",
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &Cloudreve{}
	})
}

package mopan

import (
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/op"
)

type Addition struct {
	Phone    string `json:"phone" required:"true"`
	Password string `json:"password" required:"true"`

	RootFolderID string `json:"root_folder_id" default:"-11" required:"true" help:"be careful when using the -11 value, some operations may cause system errors"`

	CloudID string `json:"cloud_id"`

	OrderBy        string `json:"order_by" type:"select" options:"filename,filesize,lastOpTime" default:"filename"`
	OrderDirection string `json:"order_direction" type:"select" options:"asc,desc" default:"asc"`

	DeviceInfo string `json:"device_info"`
}

func (a *Addition) GetRootId() string {
	return a.RootFolderID
}

var config = driver.Config{
	Name:        "MoPan",
	// DefaultRoot: "root, / or other",
	CheckStatus: true,
	Alert:       "warning|This network disk may store your password in clear text. Please set your password carefully",
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &MoPan{}
	})
}

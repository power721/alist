package aliyundrive_share

import (
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/op"
)

type Addition struct {
	RefreshToken         string `json:"RefreshToken" required:"true"`
	ShareId              string `json:"share_id" required:"true"`
	SharePwd             string `json:"share_pwd"`
	TempTransferFolderID string `json:"TempTransferFolderID" required:"true"`
	RefreshTokenOpen     string `json:"RefreshTokenOpen" required:"true"`
	OauthTokenURL        string `json:"oauth_token_url" default:"https://api.xhofe.top/alist/ali_open/token"`
	ClientID             string `json:"client_id" required:"false" help:"Keep it empty if you don't have one"`
	ClientSecret         string `json:"client_secret" required:"false" help:"Keep it empty if you don't have one"`
	driver.RootID
	DriveType      string `json:"drive_type" type:"select" options:"default,resource,backup" default:"resource"`
	OrderBy        string `json:"order_by" type:"select" options:"name,size,updated_at,created_at"`
	OrderDirection string `json:"order_direction" type:"select" options:"ASC,DESC"`
}

var config = driver.Config{
	Name:        "AliyundriveShare2Open",
	LocalSort:   false,
	OnlyProxy:   false,
	NoUpload:    true,
	DefaultRoot: "root",
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &AliyundriveShare2Open{
			base: "https://openapi.aliyundrive.com",
		}
	})
}

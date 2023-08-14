package drivers

import (
	_ "github.com/alist-org/alist/v3/drivers/alias"
	_ "github.com/alist-org/alist/v3/drivers/alist_v2"
	_ "github.com/alist-org/alist/v3/drivers/alist_v3"
	_ "github.com/alist-org/alist/v3/drivers/aliyundrive_open"
	_ "github.com/alist-org/alist/v3/drivers/aliyundrive_share2_open"
	_ "github.com/alist-org/alist/v3/drivers/local"
	_ "github.com/alist-org/alist/v3/drivers/onedrive"
	_ "github.com/alist-org/alist/v3/drivers/onedrive_app"
	_ "github.com/alist-org/alist/v3/drivers/pikpak"
	_ "github.com/alist-org/alist/v3/drivers/pikpak_share"
	_ "github.com/alist-org/alist/v3/drivers/quark_uc"
	_ "github.com/alist-org/alist/v3/drivers/thunder"
	_ "github.com/alist-org/alist/v3/drivers/url_tree"
	_ "github.com/alist-org/alist/v3/drivers/webdav"
)

// All do nothing,just for import
// same as _ import
func All() {

}

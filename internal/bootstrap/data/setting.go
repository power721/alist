package data

import (
	"github.com/alist-org/alist/v3/cmd/flags"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/offline_download/tool"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/alist-org/alist/v3/pkg/utils/random"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

var initialSettingItems []model.SettingItem

func initSettings() {
	InitialSettings()
	// check deprecated
	settings, err := op.GetSettingItems()
	if err != nil {
		utils.Log.Fatalf("failed get settings: %+v", err)
	}

	for i := range settings {
		if !isActive(settings[i].Key) && settings[i].Flag != model.DEPRECATED {
			settings[i].Flag = model.DEPRECATED
			err = op.SaveSettingItem(&settings[i])
			if err != nil {
				utils.Log.Fatalf("failed save setting: %+v", err)
			}
		}
	}

	// create or save setting
	for i := range initialSettingItems {
		item := &initialSettingItems[i]
		// err
		stored, err := op.GetSettingItemByKey(item.Key)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Log.Fatalf("failed get setting: %+v", err)
			continue
		}
		// save
		if stored != nil && item.Key != conf.VERSION {
			item.Value = stored.Value
		}
		if stored == nil || *item != *stored {
			err = op.SaveSettingItem(item)
			if err != nil {
				utils.Log.Fatalf("failed save setting: %+v", err)
			}
		} else {
			// Not save so needs to execute hook
			_, err = op.HandleSettingItemHook(item)
			if err != nil {
				utils.Log.Errorf("failed to execute hook on %s: %+v", item.Key, err)
			}
		}
	}
}

func isActive(key string) bool {
	for _, item := range initialSettingItems {
		if item.Key == key {
			return true
		}
	}
	return false
}

func InitialSettings() []model.SettingItem {
	var token string
	if flags.Dev {
		token = "dev_token"
	} else {
		token = random.Token()
	}
	initialSettingItems = []model.SettingItem{
		// site settings
		{Key: conf.VERSION, Value: conf.Version, Type: conf.TypeString, Group: model.SITE, Flag: model.READONLY},
		//{Key: conf.ApiUrl, Value: "", Type: conf.TypeString, Group: model.SITE},
		//{Key: conf.BasePath, Value: "", Type: conf.TypeString, Group: model.SITE},
		{Key: conf.SiteTitle, Value: "AList", Type: conf.TypeString, Group: model.SITE},
		{Key: conf.Announcement, Value: "### repo\nhttps://github.com/alist-org/alist", Type: conf.TypeText, Group: model.SITE},
		{Key: "pagination_type", Value: "all", Type: conf.TypeSelect, Options: "all,pagination,load_more,auto_load_more", Group: model.SITE},
		{Key: "default_page_size", Value: "30", Type: conf.TypeNumber, Group: model.SITE},
		{Key: conf.AllowIndexed, Value: "false", Type: conf.TypeBool, Group: model.SITE},
		{Key: conf.AllowMounted, Value: "true", Type: conf.TypeBool, Group: model.SITE},
		{Key: conf.RobotsTxt, Value: "User-agent: *\nAllow: /", Type: conf.TypeText, Group: model.SITE},
		// style settings
		{Key: conf.Logo, Value: "https://cdn.jsdelivr.net/gh/alist-org/logo@main/logo.svg", Type: conf.TypeText, Group: model.STYLE},
		{Key: conf.Favicon, Value: "https://cdn.jsdelivr.net/gh/alist-org/logo@main/logo.svg", Type: conf.TypeString, Group: model.STYLE},
		{Key: conf.MainColor, Value: "#1890ff", Type: conf.TypeString, Group: model.STYLE},
		{Key: "home_icon", Value: "🏠", Type: conf.TypeString, Group: model.STYLE},
		{Key: "home_container", Value: "max_980px", Type: conf.TypeSelect, Options: "max_980px,hope_container", Group: model.STYLE},
		{Key: "settings_layout", Value: "list", Type: conf.TypeSelect, Options: "list,responsive", Group: model.STYLE},
		// preview settings
		{Key: conf.TextTypes, Value: "txt,htm,html,xml,java,properties,sql,js,md,json,conf,ini,vue,php,py,bat,gitignore,yml,go,sh,c,cpp,h,hpp,tsx,vtt,srt,ass,rs,lrc", Type: conf.TypeText, Group: model.PREVIEW, Flag: model.PRIVATE},
		{Key: conf.AudioTypes, Value: "mp3,flac,ogg,m4a,wav,opus,wma", Type: conf.TypeText, Group: model.PREVIEW, Flag: model.PRIVATE},
		{Key: conf.VideoTypes, Value: "mp4,mkv,avi,mov,rmvb,webm,flv", Type: conf.TypeText, Group: model.PREVIEW, Flag: model.PRIVATE},
		{Key: conf.ImageTypes, Value: "jpg,tiff,jpeg,png,gif,bmp,svg,ico,swf,webp", Type: conf.TypeText, Group: model.PREVIEW, Flag: model.PRIVATE},
		//{Key: conf.OfficeTypes, Value: "doc,docx,xls,xlsx,ppt,pptx", Type: conf.TypeText, Group: model.PREVIEW, Flag: model.PRIVATE},
		{Key: conf.ProxyTypes, Value: "m3u8", Type: conf.TypeText, Group: model.PREVIEW, Flag: model.PRIVATE},
		{Key: conf.ProxyIgnoreHeaders, Value: "authorization,referer", Type: conf.TypeText, Group: model.PREVIEW, Flag: model.PRIVATE},
		{Key: "external_previews", Value: `{}`, Type: conf.TypeText, Group: model.PREVIEW},
		{Key: "iframe_previews", Value: `{
	"doc,docx,xls,xlsx,ppt,pptx": {
		"Microsoft":"https://view.officeapps.live.com/op/view.aspx?src=$e_url",
		"Google":"https://docs.google.com/gview?url=$e_url&embedded=true"
	},
	"pdf": {
		"PDF.js":"https://alist-org.github.io/pdf.js/web/viewer.html?file=$e_url"
	},
	"epub": {
		"EPUB.js":"https://alist-org.github.io/static/epub.js/viewer.html?url=$e_url"
	}
}`, Type: conf.TypeText, Group: model.PREVIEW},
		//		{Key: conf.OfficeViewers, Value: `{
		//	"Microsoft":"https://view.officeapps.live.com/op/view.aspx?src=$url",
		//	"Google":"https://docs.google.com/gview?url=$url&embedded=true",
		//}`, Type: conf.TypeText, Group: model.PREVIEW},
		//		{Key: conf.PdfViewers, Value: `{
		//	"pdf.js":"https://alist-org.github.io/pdf.js/web/viewer.html?file=$url"
		//}`, Type: conf.TypeText, Group: model.PREVIEW},
		{Key: "audio_cover", Value: "https://jsd.nn.ci/gh/alist-org/logo@main/logo.svg", Type: conf.TypeString, Group: model.PREVIEW},
		{Key: conf.AudioAutoplay, Value: "true", Type: conf.TypeBool, Group: model.PREVIEW},
		{Key: conf.VideoAutoplay, Value: "true", Type: conf.TypeBool, Group: model.PREVIEW},
		// global settings
		{Key: conf.HideFiles, Value: "/\\/README.md/i", Type: conf.TypeText, Group: model.GLOBAL},
		{Key: "package_download", Value: "true", Type: conf.TypeBool, Group: model.GLOBAL},
		{Key: conf.CustomizeHead, Value: `<script src="https://polyfill.io/v3/polyfill.min.js?features=String.prototype.replaceAll"></script>`, Type: conf.TypeText, Group: model.GLOBAL, Flag: model.PRIVATE},
		{Key: conf.CustomizeBody, Type: conf.TypeText, Group: model.GLOBAL, Flag: model.PRIVATE},
		{Key: conf.LinkExpiration, Value: "0", Type: conf.TypeNumber, Group: model.GLOBAL, Flag: model.PRIVATE},
		{Key: conf.SignAll, Value: "false", Type: conf.TypeBool, Group: model.GLOBAL, Flag: model.PRIVATE},
		{Key: conf.PrivacyRegs, Value: `(?:(?:\d|[1-9]\d|1\d\d|2[0-4]\d|25[0-5])\.){3}(?:\d|[1-9]\d|1\d\d|2[0-4]\d|25[0-5])
([[:xdigit:]]{1,4}(?::[[:xdigit:]]{1,4}){7}|::|:(?::[[:xdigit:]]{1,4}){1,6}|[[:xdigit:]]{1,4}:(?::[[:xdigit:]]{1,4}){1,5}|(?:[[:xdigit:]]{1,4}:){2}(?::[[:xdigit:]]{1,4}){1,4}|(?:[[:xdigit:]]{1,4}:){3}(?::[[:xdigit:]]{1,4}){1,3}|(?:[[:xdigit:]]{1,4}:){4}(?::[[:xdigit:]]{1,4}){1,2}|(?:[[:xdigit:]]{1,4}:){5}:[[:xdigit:]]{1,4}|(?:[[:xdigit:]]{1,4}:){1,6}:)
(?U)access_token=(.*)&`,
			Type: conf.TypeText, Group: model.GLOBAL, Flag: model.PRIVATE},
		{Key: conf.OcrApi, Value: "https://api.nn.ci/ocr/file/json", Type: conf.TypeString, Group: model.GLOBAL},
		{Key: conf.FilenameCharMapping, Value: `{"/": "|"}`, Type: conf.TypeText, Group: model.GLOBAL},
		{Key: conf.ForwardDirectLinkParams, Value: "false", Type: conf.TypeBool, Group: model.GLOBAL},
		{Key: conf.IgnoreDirectLinkParams, Value: "sign,alist_ts", Type: conf.TypeString, Group: model.GLOBAL},
		{Key: conf.WebauthnLoginEnabled, Value: "false", Type: conf.TypeBool, Group: model.GLOBAL, Flag: model.PUBLIC},

		// single settings
		{Key: conf.Token, Value: token, Type: conf.TypeString, Group: model.SINGLE, Flag: model.PRIVATE},
		{Key: conf.SearchIndex, Value: "none", Type: conf.TypeSelect, Options: "database,database_non_full_text,bleve,none", Group: model.INDEX},
		{Key: conf.AutoUpdateIndex, Value: "false", Type: conf.TypeBool, Group: model.INDEX},
		{Key: conf.IgnorePaths, Value: "", Type: conf.TypeText, Group: model.INDEX, Flag: model.PRIVATE, Help: `one path per line`},
		{Key: conf.MaxIndexDepth, Value: "20", Type: conf.TypeNumber, Group: model.INDEX, Flag: model.PRIVATE, Help: `max depth of index`},
		{Key: conf.IndexProgress, Value: "{}", Type: conf.TypeText, Group: model.SINGLE, Flag: model.PRIVATE},

		// SSO settings
		{Key: conf.SSOLoginEnabled, Value: "false", Type: conf.TypeBool, Group: model.SSO, Flag: model.PUBLIC},
		{Key: conf.SSOLoginPlatform, Type: conf.TypeSelect, Options: "Casdoor,Github,Microsoft,Google,Dingtalk,OIDC", Group: model.SSO, Flag: model.PUBLIC},
		{Key: conf.SSOClientId, Value: "", Type: conf.TypeString, Group: model.SSO, Flag: model.PRIVATE},
		{Key: conf.SSOClientSecret, Value: "", Type: conf.TypeString, Group: model.SSO, Flag: model.PRIVATE},
		{Key: conf.SSOOIDCUsernameKey, Value: "name", Type: conf.TypeString, Group: model.SSO, Flag: model.PRIVATE},
		{Key: conf.SSOOrganizationName, Value: "", Type: conf.TypeString, Group: model.SSO, Flag: model.PRIVATE},
		{Key: conf.SSOApplicationName, Value: "", Type: conf.TypeString, Group: model.SSO, Flag: model.PRIVATE},
		{Key: conf.SSOEndpointName, Value: "", Type: conf.TypeString, Group: model.SSO, Flag: model.PRIVATE},
		{Key: conf.SSOJwtPublicKey, Value: "", Type: conf.TypeString, Group: model.SSO, Flag: model.PRIVATE},
		{Key: conf.SSOAutoRegister, Value: "false", Type: conf.TypeBool, Group: model.SSO, Flag: model.PRIVATE},
		{Key: conf.SSODefaultDir, Value: "/", Type: conf.TypeString, Group: model.SSO, Flag: model.PRIVATE},
		{Key: conf.SSODefaultPermission, Value: "0", Type: conf.TypeNumber, Group: model.SSO, Flag: model.PRIVATE},
		{Key: conf.SSOCompatibilityMode, Value: "false", Type: conf.TypeBool, Group: model.SSO, Flag: model.PUBLIC},
		// customized settings
		{Key: conf.DeleteDelayTime, Value: "900", Type: conf.TypeNumber, Group: model.SINGLE, Flag: model.PRIVATE},
		{Key: conf.ExternalPort, Value: "5344", Type: conf.TypeNumber, Group: model.SINGLE, Flag: model.PRIVATE},
		{Key: conf.QuarkCookie, Value: "", Type: conf.TypeText, Group: model.SINGLE, Flag: model.PRIVATE},
		{Key: conf.UcCookie, Value: "", Type: conf.TypeText, Group: model.SINGLE, Flag: model.PRIVATE},
		{Key: conf.AliTo115, Value: "false", Type: conf.TypeBool, Group: model.SINGLE, Flag: model.PRIVATE},
		{Key: "open_token_url", Value: "https://api.xhofe.top/alist/ali_open/token", Type: conf.TypeString, Group: model.SINGLE, Flag: model.PRIVATE},
		{Key: "open_api_client_id", Value: "", Type: conf.TypeString, Group: model.SINGLE, Flag: model.PRIVATE},
		{Key: "open_api_client_secret", Value: "", Type: conf.TypeString, Group: model.SINGLE, Flag: model.PRIVATE},
		{Key: "ali_account_id", Value: "0", Type: conf.TypeNumber, Group: model.SINGLE, Flag: model.PRIVATE},
	}
	initialSettingItems = append(initialSettingItems, tool.Tools.Items()...)
	if flags.Dev {
		initialSettingItems = append(initialSettingItems, []model.SettingItem{
			{Key: "test_deprecated", Value: "test_value", Type: conf.TypeString, Flag: model.DEPRECATED},
			{Key: "test_options", Value: "a", Type: conf.TypeSelect, Options: "a,b,c"},
			{Key: "test_help", Type: conf.TypeString, Help: "this is a help message"},
		}...)
	}
	return initialSettingItems
}

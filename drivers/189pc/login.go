package _189pc

import (
	"errors"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/token"
	"github.com/alist-org/alist/v3/pkg/cookie"
	"strconv"
	"strings"

	"github.com/alist-org/alist/v3/pkg/utils"
	log "github.com/sirupsen/logrus"
)

type AppConf struct {
	Data struct {
		AccountType     string `json:"accountType"`
		AgreementCheck  string `json:"agreementCheck"`
		AppKey          string `json:"appKey"`
		ClientType      int    `json:"clientType"`
		IsOauth2        bool   `json:"isOauth2"`
		LoginSort       string `json:"loginSort"`
		MailSuffix      string `json:"mailSuffix"`
		PageKey         string `json:"pageKey"`
		ParamId         string `json:"paramId"`
		RegReturnUrl    string `json:"regReturnUrl"`
		ReqId           string `json:"reqId"`
		ReturnUrl       string `json:"returnUrl"`
		ShowFeedback    string `json:"showFeedback"`
		ShowPwSaveName  string `json:"showPwSaveName"`
		ShowQrSaveName  string `json:"showQrSaveName"`
		ShowSmsSaveName string `json:"showSmsSaveName"`
		Sso             string `json:"sso"`
	} `json:"data"`
	Msg    string `json:"msg"`
	Result string `json:"result"`
}

type EncryptConf struct {
	Result int `json:"result"`
	Data   struct {
		UpSmsOn   string `json:"upSmsOn"`
		Pre       string `json:"pre"`
		PreDomain string `json:"preDomain"`
		PubKey    string `json:"pubKey"`
	} `json:"data"`
}

func (d *Cloud189PC) newLogin() error {
	url := "https://cloud.189.cn/api/portal/loginUrl.action?redirectURL=https%3A%2F%2Fcloud.189.cn%2Fweb%2Fredirect.html&defaultSaveName=3&defaultSaveNameCheck=uncheck&browserId=feb1dc2431c6660d064c3edde0ab7e9d"
	res, err := d.client.R().Get(url)
	if err != nil {
		return err
	}
	// Is logged in
	redirectURL := res.RawResponse.Request.URL
	if redirectURL.String() == "https://cloud.189.cn/web/main" {
		return nil
	}
	lt := redirectURL.Query().Get("lt")
	reqId := redirectURL.Query().Get("reqId")
	appId := redirectURL.Query().Get("appId")
	headers := map[string]string{
		"lt":         lt,
		"reqid":      reqId,
		"referer":    redirectURL.String(),
		"origin":     "https://open.e.189.cn",
		"user-agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36",
	}

	// get app Conf
	var appConf AppConf
	res, err = d.client.R().SetHeaders(headers).SetFormData(map[string]string{
		"version": "2.0",
		"appKey":  appId,
	}).SetResult(&appConf).Post("https://open.e.189.cn/api/logbox/oauth2/appConf.do")
	if err != nil {
		return err
	}
	log.Debugf("189 AppConf resp body: %s", res.String())
	if appConf.Result != "0" {
		return errors.New(appConf.Msg)
	}
	// get encrypt conf
	var encryptConf EncryptConf
	res, err = d.client.R().SetHeaders(headers).SetFormData(map[string]string{
		"appId": appId,
	}).Post("https://open.e.189.cn/api/logbox/config/encryptConf.do")
	if err != nil {
		return err
	}
	err = utils.Json.Unmarshal(res.Body(), &encryptConf)
	if err != nil {
		return err
	}
	log.Debugf("189 EncryptConf resp body: %s\n%+v", res.String(), encryptConf)
	if encryptConf.Result != 0 {
		return errors.New("get EncryptConf error:" + res.String())
	}
	// TODO: getUUID? needcaptcha
	// login
	loginData := map[string]string{
		"version":         "v2.0",
		"apToken":         "",
		"appKey":          appId,
		"accountType":     appConf.Data.AccountType,
		"userName":        encryptConf.Data.Pre + RsaEncode([]byte(d.Username), encryptConf.Data.PubKey, true),
		"epd":             encryptConf.Data.Pre + RsaEncode([]byte(d.Password), encryptConf.Data.PubKey, true),
		"captchaType":     "",
		"validateCode":    "",
		"smsValidateCode": "",
		"captchaToken":    "",
		"returnUrl":       appConf.Data.ReturnUrl,
		"mailSuffix":      appConf.Data.MailSuffix,
		"dynamicCheck":    "FALSE",
		"clientType":      strconv.Itoa(appConf.Data.ClientType),
		"cb_SaveName":     "3",
		"isOauth2":        strconv.FormatBool(appConf.Data.IsOauth2),
		"state":           "",
		"paramId":         appConf.Data.ParamId,
	}
	res, err = d.client.R().SetHeaders(headers).SetFormData(loginData).Post("https://open.e.189.cn/api/logbox/oauth2/loginSubmit.do")
	if err != nil {
		return err
	}
	log.Debugf("189 login resp body: %s", res.String())
	loginResult := utils.Json.Get(res.Body(), "result").ToInt()
	if loginResult != 0 {
		return errors.New(utils.Json.Get(res.Body(), "msg").ToString())
	}

	ToUrl := utils.Json.Get(res.Body(), "toUrl").ToString()
	res, err = d.client2.R().
		SetHeader("accept", "*/*").
		SetHeader("referer", "https://open.e.189.cn/").
		Get(ToUrl)
	if err != nil {
		return err
	}
	sCookie := cookie.ToString(res.Cookies())
	if strings.Contains(sCookie, "JSESSIONID") && strings.Contains(sCookie, "COOKIE_LOGIN_USER") {
		log.Infof("Got 189 cookie")
		d.Cookie = sCookie
		token.SaveAccountToken(conf.CLOUD189, d.Cookie, int(d.ID))
	}

	return nil
}

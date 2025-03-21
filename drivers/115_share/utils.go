package _115_share

import (
	"fmt"
	_115 "github.com/alist-org/alist/v3/drivers/115"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/op"
	"strconv"
	"time"

	driver115 "github.com/SheltonZhu/115driver/pkg/driver"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/pkg/errors"
)

var _ model.Obj = (*FileObj)(nil)

var initialized = false
var Cookie = ""
var QRCodeToken = ""
var client *driver115.Pan115Client

type FileObj struct {
	Size     int64
	Sha1     string
	Utm      time.Time
	FileName string
	isDir    bool
	FileID   string
}

func (f *FileObj) CreateTime() time.Time {
	return f.Utm
}

func (f *FileObj) GetHash() utils.HashInfo {
	return utils.NewHashInfo(utils.SHA1, f.Sha1)
}

func (f *FileObj) GetSize() int64 {
	return f.Size
}

func (f *FileObj) GetName() string {
	return f.FileName
}

func (f *FileObj) ModTime() time.Time {
	return f.Utm
}

func (f *FileObj) IsDir() bool {
	return f.isDir
}

func (f *FileObj) GetID() string {
	return f.FileID
}

func (f *FileObj) GetPath() string {
	return ""
}

func transFunc(sf driver115.ShareFile) (model.Obj, error) {
	timeInt, err := strconv.ParseInt(sf.UpdateTime, 10, 64)
	if err != nil {
		return nil, err
	}
	var (
		utm    = time.Unix(timeInt, 0)
		isDir  = (sf.IsFile == 0)
		fileID = string(sf.FileID)
	)
	if isDir {
		fileID = string(sf.CategoryID)
	}
	return &FileObj{
		Size:     int64(sf.Size),
		Sha1:     sf.Sha1,
		Utm:      utm,
		FileName: string(sf.FileName),
		isDir:    isDir,
		FileID:   fileID,
	}, nil
}

var UserAgent = conf.UA115Browser

func (d *Pan115Share) login() error {
	var err error
	opts := []driver115.Option{
		driver115.UA(UserAgent),
	}
	client = driver115.New(opts...)
	if _, err := client.GetShareSnap(d.ShareCode, d.ReceiveCode, ""); err != nil {
		return errors.Wrap(err, "failed to get share snap")
	}
	cr := &driver115.Credential{}
	pan115 := op.GetFirst115Driver()
	if pan115 != nil {
		QRCodeToken = pan115.(*_115.Pan115).QRCodeToken
		Cookie = pan115.(*_115.Pan115).Cookie
	} else {
		return errors.New("115 Cloud not init")
	}
	if QRCodeToken != "" {
		s := &driver115.QRCodeSession{
			UID: QRCodeToken,
		}
		if cr, err = client.QRCodeLogin(s); err != nil {
			return errors.Wrap(err, "failed to login by qrcode")
		}
		Cookie = fmt.Sprintf("UID=%s;CID=%s;SEID=%s;KID=%s", cr.UID, cr.CID, cr.SEID, cr.KID)
		QRCodeToken = ""
	} else if Cookie != "" {
		if err = cr.FromCookie(Cookie); err != nil {
			return errors.Wrap(err, "failed to login by cookies")
		}
		client.ImportCredential(cr)
	} else {
		return errors.New("missing cookie or qrcode account")
	}

	return client.LoginCheck()
}

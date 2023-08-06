package aliyundrive_share2_open

import (
	"time"

	"github.com/alist-org/alist/v3/internal/model"
)

type ErrorResp struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ShareTokenResp struct {
	ShareToken string    `json:"share_token"`
	ExpireTime time.Time `json:"expire_time"`
	ExpiresIn  int       `json:"expires_in"`
}

type ListResp struct {
	Items             []File `json:"items"`
	NextMarker        string `json:"next_marker"`
	PunishedFileCount int    `json:"punished_file_count"`
}

type File struct {
	ID           string    `json:"id"`
	DriveId      string    `json:"drive_id"`
	DomainId     string    `json:"domain_id"`
	FileId       string    `json:"file_id"`
	ShareId      string    `json:"share_id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ParentFileId string    `json:"parent_file_id"`
	Size         int64     `json:"size"`
	Thumbnail    string    `json:"thumbnail"`
}

func fileToObj(f File) *model.ObjThumb {
	return &model.ObjThumb{
		Object: model.Object{
			ID:       f.FileId,
			Name:     f.Name,
			Size:     f.Size,
			Modified: f.UpdatedAt,
			IsFolder: f.Type == "folder",
		},
		Thumbnail: model.Thumbnail{Thumbnail: f.Thumbnail},
	}
}

type ShareLinkResp struct {
	DownloadUrl string `json:"download_url"`
	Url         string `json:"url"`
	Thumbnail   string `json:"thumbnail"`
}

type Request struct {
}

type MyFile struct {
	FileId   string    `json:"file_id"`
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	UpdateAt time.Time `json:"UpdateAt"`
}

func (f MyFile) GetPath() string {
	return ""
}

func (f MyFile) GetSize() int64 {
	return f.Size
}

func (f MyFile) GetName() string {
	return f.Name
}

func (f MyFile) ModTime() time.Time {
	return f.UpdateAt
}

func (f MyFile) IsDir() bool {
	return false
}

func (f MyFile) GetID() string {
	return f.FileId
}

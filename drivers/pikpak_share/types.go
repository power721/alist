package pikpak_share

import (
	"strconv"
	"time"

	"github.com/alist-org/alist/v3/internal/model"
)

type RespErr struct {
	ErrorCode int    `json:"error_code"`
	Error     string `json:"error"`
}

type ShareResp struct {
	ShareStatus		string `json:"share_status"`
	ShareStatusText	string `json:"share_status_text"`
	FileInfo        File   `json:"file_info"`
	Files           []File `json:"files"`
	NextPageToken   string `json:"next_page_token"`
	PassCodeToken   string `json:"pass_code_token"`
}

type File struct {
	Id             string    `json:"id"`
	ShareId        string    `json:"share_id"`
	Kind           string    `json:"kind"`
	Name           string    `json:"name"`
	ModifiedTime   time.Time `json:"modified_time"`
	Size           string    `json:"size"`
	ThumbnailLink  string    `json:"thumbnail_link"`
	WebContentLink string    `json:"web_content_link"`
	Medias         []Media   `json:"medias"`
}

func fileToObj(f File) *model.ObjThumb {
	size, _ := strconv.ParseInt(f.Size, 10, 64)
	return &model.ObjThumb{
		Object: model.Object{
			ID:       f.Id,
			Name:     f.Name,
			Size:     size,
			Modified: f.ModifiedTime,
			IsFolder: f.Kind == "drive#folder",
		},
		Thumbnail: model.Thumbnail{
			Thumbnail: f.ThumbnailLink,
		},
	}
}

type Media struct {
	MediaId   string `json:"media_id"`
	MediaName string `json:"media_name"`
	Video     struct {
		Height     int    `json:"height"`
		Width      int    `json:"width"`
		Duration   int    `json:"duration"`
		BitRate    int    `json:"bit_rate"`
		FrameRate  int    `json:"frame_rate"`
		VideoCodec string `json:"video_codec"`
		AudioCodec string `json:"audio_codec"`
		VideoType  string `json:"video_type"`
	} `json:"video"`
	Link struct {
		Url    string    `json:"url"`
		Token  string    `json:"token"`
		Expire time.Time `json:"expire"`
	} `json:"link"`
	NeedMoreQuota  bool          `json:"need_more_quota"`
	VipTypes       []interface{} `json:"vip_types"`
	RedirectLink   string        `json:"redirect_link"`
	IconLink       string        `json:"icon_link"`
	IsDefault      bool          `json:"is_default"`
	Priority       int           `json:"priority"`
	IsOrigin       bool          `json:"is_origin"`
	ResolutionName string        `json:"resolution_name"`
	IsVisible      bool          `json:"is_visible"`
	Category       string        `json:"category"`
}

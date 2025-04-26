package conf

const (
	TypeString = "string"
	TypeSelect = "select"
	TypeBool   = "bool"
	TypeText   = "text"
	TypeNumber = "number"
)

const (
	UA115Browser = "Mozilla/5.0 115Browser/27.0.3.7"
	UserAgent    = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"
	// site
	VERSION      = "version"
	SiteTitle    = "site_title"
	Announcement = "announcement"
	AllowIndexed = "allow_indexed"
	AllowMounted = "allow_mounted"
	RobotsTxt    = "robots_txt"

	Logo      = "logo"
	Favicon   = "favicon"
	MainColor = "main_color"

	// preview
	TextTypes                = "text_types"
	AudioTypes               = "audio_types"
	VideoTypes               = "video_types"
	ImageTypes               = "image_types"
	ProxyTypes               = "proxy_types"
	ProxyIgnoreHeaders       = "proxy_ignore_headers"
	AudioAutoplay            = "audio_autoplay"
	VideoAutoplay            = "video_autoplay"
	PreviewArchivesByDefault = "preview_archives_by_default"
	ReadMeAutoRender         = "readme_autorender"
	FilterReadMeScripts      = "filter_readme_scripts"
	// global
	HideFiles               = "hide_files"
	CustomizeHead           = "customize_head"
	CustomizeBody           = "customize_body"
	LinkExpiration          = "link_expiration"
	SignAll                 = "sign_all"
	PrivacyRegs             = "privacy_regs"
	OcrApi                  = "ocr_api"
	FilenameCharMapping     = "filename_char_mapping"
	ForwardDirectLinkParams = "forward_direct_link_params"
	IgnoreDirectLinkParams  = "ignore_direct_link_params"
	WebauthnLoginEnabled    = "webauthn_login_enabled"

	// index
	SearchIndex     = "search_index"
	AutoUpdateIndex = "auto_update_index"
	IgnorePaths     = "ignore_paths"
	MaxIndexDepth   = "max_index_depth"

	// aria2
	Aria2Uri    = "aria2_uri"
	Aria2Secret = "aria2_secret"

	// transmission
	TransmissionUri      = "transmission_uri"
	TransmissionSeedtime = "transmission_seedtime"

	// 115
	Pan115TempDir = "115_temp_dir"

	// pikpak
	PikPakTempDir = "pikpak_temp_dir"

	// thunder
	ThunderTempDir = "thunder_temp_dir"

	// single
	Token           = "token"
	IndexProgress   = "index_progress"
	DeleteDelayTime = "delete_delay_time"
	ExternalPort    = "external_port"
	QUARK           = "QUARK"
	UC              = "UC"
	PAN115          = "PAN115"
	OPEN115         = "OPEN115"
	PAN139          = "PAN139"
	AliTo115        = "ali_to_115"

	// SSO
	SSOClientId          = "sso_client_id"
	SSOClientSecret      = "sso_client_secret"
	SSOLoginEnabled      = "sso_login_enabled"
	SSOLoginPlatform     = "sso_login_platform"
	SSOOIDCUsernameKey   = "sso_oidc_username_key"
	SSOOrganizationName  = "sso_organization_name"
	SSOApplicationName   = "sso_application_name"
	SSOEndpointName      = "sso_endpoint_name"
	SSOJwtPublicKey      = "sso_jwt_public_key"
	SSOExtraScopes       = "sso_extra_scopes"
	SSOAutoRegister      = "sso_auto_register"
	SSODefaultDir        = "sso_default_dir"
	SSODefaultPermission = "sso_default_permission"
	SSOCompatibilityMode = "sso_compatibility_mode"

	// ldap
	LdapLoginEnabled      = "ldap_login_enabled"
	LdapServer            = "ldap_server"
	LdapManagerDN         = "ldap_manager_dn"
	LdapManagerPassword   = "ldap_manager_password"
	LdapUserSearchBase    = "ldap_user_search_base"
	LdapUserSearchFilter  = "ldap_user_search_filter"
	LdapDefaultPermission = "ldap_default_permission"
	LdapDefaultDir        = "ldap_default_dir"
	LdapLoginTips         = "ldap_login_tips"

	// s3
	S3Buckets         = "s3_buckets"
	S3AccessKeyId     = "s3_access_key_id"
	S3SecretAccessKey = "s3_secret_access_key"

	// qbittorrent
	QbittorrentUrl      = "qbittorrent_url"
	QbittorrentSeedtime = "qbittorrent_seedtime"

	// ftp
	FTPPublicHost        = "ftp_public_host"
	FTPPasvPortMap       = "ftp_pasv_port_map"
	FTPProxyUserAgent    = "ftp_proxy_user_agent"
	FTPMandatoryTLS      = "ftp_mandatory_tls"
	FTPImplicitTLS       = "ftp_implicit_tls"
	FTPTLSPrivateKeyPath = "ftp_tls_private_key_path"
	FTPTLSPublicCertPath = "ftp_tls_public_cert_path"

	// traffic
	TaskOfflineDownloadThreadsNum         = "offline_download_task_threads_num"
	TaskOfflineDownloadTransferThreadsNum = "offline_download_transfer_task_threads_num"
	TaskUploadThreadsNum                  = "upload_task_threads_num"
	TaskCopyThreadsNum                    = "copy_task_threads_num"
	TaskDecompressDownloadThreadsNum      = "decompress_download_task_threads_num"
	TaskDecompressUploadThreadsNum        = "decompress_upload_task_threads_num"
	StreamMaxClientDownloadSpeed          = "max_client_download_speed"
	StreamMaxClientUploadSpeed            = "max_client_upload_speed"
	StreamMaxServerDownloadSpeed          = "max_server_download_speed"
	StreamMaxServerUploadSpeed            = "max_server_upload_speed"
)

const (
	UNKNOWN = iota
	FOLDER
	// OFFICE
	VIDEO
	AUDIO
	TEXT
	IMAGE
)

// ContextKey is the type of context keys.
const (
	NoTaskKey = "no_task"
)

const (
	AliThreads     = 4
	AliChunkSize   = 512
	UcThreads      = 8
	UcChunkSize    = 1024
	QuarkThreads   = 10
	QuarkChunkSize = 1024
)

package protocol

type Config struct {
	Username string
	Password string
	IP       [4]byte
	MAC      [6]byte
	HostName string
	OSInfo   string

	PrimaryDNS [4]byte
	DHCPServer [4]byte

	AuthVersion           [2]byte
	KeepAliveVersion      [2]byte
	FirstHeartbeatVersion [2]byte
	ExtraHeartbeatVersion [2]byte
}

type Session struct {
	LoginSalt            [4]byte
	LogoutSalt           [4]byte
	MD5Password          [16]byte
	ServerDrcomIndicator [16]byte
	HeartbeatToken       [4]byte
	HeartbeatCount       uint64
}

const (
	SizeChallenge          = 20
	SizeKeepAliveAuth      = 38
	SizeKeepAliveHeartbeat = 40
	SizeLogout             = 80
)

const (
	loginOffsetMD5A               = 4
	loginOffsetUsername           = 20
	loginOffsetControlCheckStatus = 56
	loginOffsetAdapterNum         = 57
	loginOffsetMacXorMD5          = 58
	loginOffsetMD5B               = 64
	loginOffsetIPIndicator        = 80
	loginOffsetIP                 = 81
	loginOffsetMD5C               = 97
	loginOffsetIPDog              = 105
	loginOffsetHostName           = 110
	loginOffsetPrimaryDNS         = 142
	loginOffsetDHCPServer         = 146
	loginOffsetDrcomFlag          = 181
	loginOffsetDrcomIndicator     = 182
	loginOffsetAuthVersionA       = 190
	loginOffsetOSInfo             = 192
	loginOffsetUnknownIndicator   = 246
	loginOffsetAuthVersionB       = 310
	loginOffsetPasswordLength     = 313
	loginOffsetPasswordROR        = 314
)

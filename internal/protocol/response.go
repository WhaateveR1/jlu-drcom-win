package protocol

import (
	"encoding/binary"
	"errors"
)

var ErrRejected = errors.New("server rejected request")

// ResponseMatcher identifies the reply belonging to this request. Payloads are
// never included in errors: several replies contain reusable session secrets.
func ResponseMatcher(request []byte) func([]byte) (bool, error) {
	return func(p []byte) (bool, error) {
		if len(p) == 0 || len(request) == 0 {
			return false, nil
		}
		if p[0] == 0x05 {
			return false, ErrRejected
		}
		switch request[0] {
		case 0x01:
			return len(request) >= 2 && len(p) >= 8 && p[0] == 0x02 && p[1] == request[1], nil
		case 0x03:
			return len(p) >= 39 && p[0] == 0x04, nil
		case 0x06:
			return ParseLogoutResponse(p) == nil, nil
		case 0xff:
			return ParseKeepAliveAuthResponse(p) == nil, nil
		case 0x07:
			if len(request) < 8 || !validHeartbeat(p) || p[1] != request[1] {
				return false, nil
			}
			if request[5] == 0x03 {
				return p[5] == 0x04, nil
			}
			// First/extra requests can return the server's file/update packet.
			if request[6] == 0x0f || request[6] == 0xdb {
				return p[5] == 0x06 || p[5] == 0x02, nil
			}
			return p[5] == 0x02, nil
		}
		return false, nil
	}
}

func validHeartbeat(p []byte) bool {
	if len(p) < 40 || p[0] != 0x07 || p[4] != 0x0b || int(binary.LittleEndian.Uint16(p[2:4])) != len(p) {
		return false
	}
	return p[5] == 0x02 || p[5] == 0x04 || p[5] == 0x06
}

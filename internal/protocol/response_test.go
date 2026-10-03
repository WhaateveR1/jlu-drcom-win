package protocol

import "testing"

func TestResponseMatcherRejectsWrongPhaseSequenceAndTruncation(t *testing.T) {
	req := []byte{7, 9, 40, 0, 0x0b, 1, 0xdc, 2}
	good := make([]byte, 40)
	copy(good, []byte{7, 9, 40, 0, 0x0b, 2})
	match := ResponseMatcher(req, false)
	if ok, err := match(good); !ok || err != nil {
		t.Fatal(ok, err)
	}
	for _, at := range []int{0, 1, 2, 4, 5} {
		bad := append([]byte(nil), good...)
		bad[at]++
		if ok, _ := match(bad); ok {
			t.Fatal("accepted corrupt header", at)
		}
	}
	for n := 0; n < 40; n++ {
		if ok, _ := match(good[:n]); ok {
			t.Fatal("accepted truncated", n)
		}
	}
	if _, err := match([]byte{5}); err != ErrRejected {
		t.Fatal(err)
	}
}

func TestKeepaliveAndLogoutObservedShapes(t *testing.T) {
	p := make([]byte, 72)
	copy(p, []byte{7, 1, 16, 0, 6, 0})
	if err := ParseKeepAliveAuthResponse(p); err != nil {
		t.Fatal(err)
	}
	l := make([]byte, 25)
	l[0] = 4
	if err := ParseLogoutResponse(l); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{nil, {6}, {7}, {0xff}, make([]byte, 40)} {
		if ParseKeepAliveAuthResponse(bad) == nil || ParseLogoutResponse(bad) == nil || ParseHeartbeatAck(bad) == nil {
			t.Fatal("accepted malformed response")
		}
	}
}

func FuzzResponseMatcher(f *testing.F) {
	f.Add([]byte{7, 0, 40, 0, 11, 1, 0xdc, 2}, []byte{5})
	f.Fuzz(func(t *testing.T, request, response []byte) {
		ResponseMatcher(request, false)(response)
		ResponseMatcher(request, true)(response)
	})
}

func TestFileResponseUsesRequestRoleNotHardcodedVersion(t *testing.T) {
	request := []byte{7, 21, 40, 0, 11, 1, 0xab, 0xcd}
	p := make([]byte, 272)
	copy(p, []byte{7, 21, 0x10, 1, 11, 6})
	if ok, err := ResponseMatcher(request, true)(p); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if ok, _ := ResponseMatcher(request, false)(p); ok {
		t.Fatal("regular heartbeat consumed a file response")
	}
}

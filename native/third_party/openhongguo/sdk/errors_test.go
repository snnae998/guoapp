package sdk

import (
	"time"

	"errors"
	"testing"

	"github.com/beilunyang/openhongguo/internal/protocol"
)

func TestAccountAndDRMErrorsStayDistinct(t *testing.T) {
	if errors.Is(ErrAuthenticationRequired, ErrDRMProtected) {
		t.Fatal("sentinels overlap")
	}
	auth := &APIError{HTTPStatus: 401, Code: "auth_required"}
	if !errors.Is(auth, ErrAuthenticationRequired) || errors.Is(auth, ErrDRMProtected) {
		t.Fatalf("auth classification failed: %v", auth)
	}
	if stringsContains(auth.Error(), "Bearer") || stringsContains(auth.Error(), "token") {
		t.Fatalf("api error text = %s", auth.Error())
	}
}

func stringsContains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (s == sub || len(s) > 0 && contains(s, sub))
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestCodeErrorMapsToOneSentinel(t *testing.T) {
	err := &CodeError{Code: 101108, Message: "请先登录", Kind: protocol.KindAuth}
	if !errors.Is(err, ErrAuthenticationRequired) {
		t.Fatal("auth code did not map to ErrAuthenticationRequired")
	}
	if errors.Is(err, ErrContentUnavailable) || Retryable(err) {
		t.Fatal("auth code classified as retryable or content")
	}
	gone := &CodeError{Code: 101001, Message: "合集已下架", Kind: protocol.KindContentGone}
	if !errors.Is(gone, ErrContentUnavailable) || Retryable(gone) {
		t.Fatal("content code classification is wrong")
	}
	limited := &CodeError{Code: 100, Message: `{"minBanTime":10,"maxBanTime":30}`, Kind: protocol.KindRateLimited}
	if !Retryable(limited) {
		t.Fatal("rate limit should be retryable")
	}
	if wait := RetryAfter(limited, time.Second); wait < 10*time.Second || wait > 30*time.Second {
		t.Fatalf("retry after = %v", wait)
	}
	if wait := RetryAfter(gone, 5*time.Second); wait != 5*time.Second {
		t.Fatalf("fallback retry after = %v", wait)
	}
}

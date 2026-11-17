package protocol

import (
	"encoding/json"
	"strings"
	"time"
)

// ErrorKind classifies a service failure so callers can decide whether to
// retry, sign in, or give up. It follows the shipped client's handling
// (NetReqUtil.assertRspDataOk and the BookApiERR table).
type ErrorKind int

const (
	KindUnknown        ErrorKind = iota
	KindAuth                     // needs an account (101108, 101123, 4001, 101009)
	KindRateLimited              // 100 FAST_REJECT with a ban window, 101143, 101144
	KindContentGone              // 101104/101109/101121/101137/101145/101115/101001
	KindParam                    // 100103, 4000, 101142
	KindChallenge                // 111 HIT_VERIFY_CODE
	KindClientRejected           // 112 APP_REJECT
	KindDeviceRejected           // BaseResp 263175/263176
	KindEndOfList                // 101 ALL_CONTENT_SHOWN: paging is done, not a failure
)

// String names the kind for logs and errors.
func (k ErrorKind) String() string {
	switch k {
	case KindUnknown:
		return "unknown"
	case KindAuth:
		return "auth"
	case KindRateLimited:
		return "rate_limited"
	case KindContentGone:
		return "content_gone"
	case KindParam:
		return "param"
	case KindChallenge:
		return "challenge"
	case KindClientRejected:
		return "client_rejected"
	case KindDeviceRejected:
		return "device_rejected"
	case KindEndOfList:
		return "end_of_list"
	}
	return "unknown"
}

// codeNames are the observed BookApiERR values. The service emits codes
// outside this table too (101001 among them), so Classify falls back to
// ranges and message keywords.
var codeNames = map[int]string{
	0:      "SUCCESS",
	100:    "FAST_REJECT",
	101:    "ALL_CONTENT_SHOWN",
	110:    "ILLEGAL_ACCESS",
	111:    "HIT_VERIFY_CODE",
	112:    "APP_REJECT",
	4000:   "PARAM_INVALID",
	4001:   "USER_NOT_LOGIN",
	7001:   "MAKE_ORDER_NO_ERROR",
	100103: "PARAM_INVALID",
	101009: "USER_NO_PERMISSION",
	101101: "REQ_ID_GET_ERROR",
	101102: "OUT_OF_MAX_LIMIT_ERROR",
	101103: "DIRECTORY_GET_ERROR",
	101104: "BOOK_NOT_EXIST_ERROR",
	101105: "BOOKSHELF_ADD_ERROR",
	101106: "BOOKSHELF_CHECK_ERROR",
	101107: "BOOKSHELF_DELETE_ERROR",
	101108: "USER_NOT_LOGIN",
	101109: "BOOK_REMOVE",
	101110: "READ_HISTORY_UPLOAD_ERROR",
	101111: "READ_HISTORY_DELETE_ERROR",
	101112: "READ_HISTORY_GET_ERROR",
	101113: "CHAPTER_INFO_GET_ERROR",
	101114: "PACK_SERVICE_ERROR",
	101115: "PERMISSION_NOT_SATISFIED",
	101116: "SERVICE_ERROR",
	101117: "ERR_ARTICLE_SERVICE_ERROR",
	101118: "BOOKSHELF_ADD_TOO_MUCH",
	101119: "BOOKSHELF_GET_ERROR",
	101120: "BOOKLIST_NAME_DUPLICATE",
	101121: "BOOK_FULLLY_REMOVE",
	101122: "SHARE_CODE_NOT_VALID",
	101123: "NOT_AUTHORIZED",
	101124: "SSTIMOR_EMPTY_ERROR",
	101130: "BOOKSHELF_TOPPING_DATA_EXPIRE",
	101131: "TRANSFER_LINE_POSITION_ERROR",
	101132: "PANIC",
	101133: "ITEM_UNLOCK_SET_ERROR",
	101134: "BOOKSHELF_ASTERISK_ERROR",
	101135: "SMART_PLAYER_SERVICE_ERROR",
	101136: "PLAY_URL_OUTTIME",
	101137: "CONTENT_VERIFYING",
	101138: "TOO_MANY_STREAM_TASK",
	101139: "NO_THIS_TONE",
	101140: "QUERY_CHAPTER_LOCK_ERROR",
	101141: "VIDEO_NO_PERMISSION",
	101142: "USER_INPUT_INVALID",
	101143: "FREQUENCY_BLOCK",
	101144: "HIT_SHARK_RULE",
	101145: "BOOK_NOT_EXIST_OVERSEA",
	101146: "ABASE_READ_FAIL",
	101147: "ABASE_WRITE_FAIL",
	101148: "UPLOAD_EMPTY_FILE",
	101149: "PARSE_FILE_ERROR",
}

// CodeName returns the shipped enum name of a known code.
func CodeName(code int) string {
	if n, ok := codeNames[code]; ok {
		return n
	}
	return ""
}

// Classify maps one business code (and its message) onto an ErrorKind.
//
// Order matters: exact entries first, then ranges the client does not name
// (1010xx is the content domain, e.g. 101001 "合集已下架"), then keywords.
func Classify(code int, message string) ErrorKind {
	switch code {
	case 0:
		return KindUnknown
	case 101:
		return KindEndOfList
	case 100, 101143, 101144, 1000003:
		return KindRateLimited
	case 101108, 101123, 4001, 101009:
		return KindAuth
	case 101104, 101109, 101121, 101137, 101145, 101115:
		return KindContentGone
	case 100103, 4000, 101142:
		return KindParam
	case 110:
		return KindParam
	case 111:
		return KindChallenge
	case 112:
		return KindClientRejected
	case 101101, 101102, 101103, 101105, 101106, 101107, 101110, 101111, 101112,
		101113, 101114, 101116, 101117, 101118, 101119, 101120, 101122, 101124,
		101130, 101131, 101132, 101133, 101134, 101135, 101136, 101138, 101139,
		101140, 101141, 101146, 101147, 101148, 101149:
		return KindContentGone
	}
	// 1010xx is the content domain; the client names only part of it.
	if code >= 101000 && code < 101200 {
		return KindContentGone
	}
	switch strings.ToLower(strings.TrimSpace(message)) {
	case "", "success":
		return KindUnknown
	}
	lower := strings.ToLower(message)
	for _, kw := range []string{"登录", "login", "sign in"} {
		if strings.Contains(lower, kw) {
			return KindAuth
		}
	}
	for _, kw := range []string{"频繁", "限流", "too many", "frequent", "rate limit"} {
		if strings.Contains(lower, kw) {
			return KindRateLimited
		}
	}
	for _, kw := range []string{"下架", "不存在", "已删除", "removed", "not exist", "unavailable"} {
		if strings.Contains(lower, kw) {
			return KindContentGone
		}
	}
	return KindUnknown
}

// BanWindow reads the SvrBanApiModel payload the service puts in the message of
// a 100 FAST_REJECT response. It returns the wait window the client should
// honour before retrying that path.
func BanWindow(message string) (minWait, maxWait time.Duration, ok bool) {
	message = strings.TrimSpace(message)
	if !strings.HasPrefix(message, "{") {
		return 0, 0, false
	}
	var ban struct {
		MinBanTime flexInt64 `json:"minBanTime"`
		MaxBanTime flexInt64 `json:"maxBanTime"`
		BanRate    float64   `json:"banRate"`
	}
	if err := json.Unmarshal([]byte(message), &ban); err != nil {
		return 0, 0, false
	}
	minWait = time.Duration(int64(ban.MinBanTime)) * time.Second
	maxWait = time.Duration(int64(ban.MaxBanTime)) * time.Second
	if minWait <= 0 && maxWait <= 0 {
		return 0, 0, false
	}
	return minWait, maxWait, true
}

// BaseStatus reads BaseResp.StatusCode, which is where the service reports a
// rejected device (263175) or installation (263176).
func BaseStatus(raw []byte) int {
	var env struct {
		BaseResp struct {
			StatusCode flexInt `json:"StatusCode"`
		} `json:"BaseResp"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0
	}
	return int(env.BaseResp.StatusCode)
}

// DeviceRejected reports the two BaseResp codes that mean the anonymous device
// must be re-registered.
func DeviceRejected(status int) bool { return status == 263175 || status == 263176 }

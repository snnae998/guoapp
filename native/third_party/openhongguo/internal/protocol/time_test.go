package protocol

import "time"

func fixtureTime() time.Time { return time.UnixMilli(1_700_000_000_123).UTC() }

package notify

import "github.com/rodjunger/nkmonitor"

type NoopNotifyer struct{}

func (n NoopNotifyer) Notify(info nkmonitor.RestockInfo) error { return nil }

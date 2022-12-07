package notify

import "github.com/rodjunger/nkmonitor"

type Notifyer interface {
	Notify(info nkmonitor.RestockInfo) error
}

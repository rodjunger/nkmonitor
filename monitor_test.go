package nkmonitor

import (
	"testing"
	"time"

	"github.com/saucesteals/mimic"
	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"
)

var validUrl = "https://www.nike.com.br/snkrs/jacket-024491.html?cor=ND"

var invalidUrls = []struct {
	url    string
	reason string
}{
	{"", "empty string"},
	{"https://nike.com.br", "no path"},
	{"https://www.youtube.com/has/a/path", "invalid domain"},
}

func TestMonitor(t *testing.T) {
	m, _ := mimic.Chromium(mimic.BrandChrome, "106.0.0.0")
	validCh := make(chan RestockInfo)

	_, err := NewMonitor("", time.Second, nil, m)
	assert.Error(t, err, "empty useragent is invalid")

	_, err = NewMonitor("not empty", time.Second, nil, nil)
	assert.Error(t, err, "nil mimic is invalid")

	fake := &Monitor{started: &atomic.Bool{}}
	_, err = fake.AddTask(validUrl, validCh)
	assert.ErrorIs(t, err, ErrNotStarted, "adding task on a monitor that is not started should return errNotStarted")

	fake.started.Store(true)
	_, err = fake.AddTask(validUrl, nil)
	assert.ErrorIs(t, err, ErrNilCallback, "adding task with nil callback should return errNilCallback")

	for _, test := range invalidUrls {
		_, err = fake.AddTask(test.url, validCh)
		assert.ErrorIsf(t, err, ErrInvalidUrl, "invalid url should return errInvalidUrl, url: %s, reason: %s", test.url, test.reason)
	}

}

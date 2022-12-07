package proxy

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type invalidProxy struct {
	url    string
	reason string
}

var invalidProxies = []invalidProxy{
	{"127.0.0.1:8080:", "two :"},
	{"", "empty"},
	{"@", "no host"},
	{"@@", "two @s"},
	{":::", "empty info"},
	{":8080", "empty host"},
	{"127.0.0.1:8080/path/to/somewhere", "has path"},
}

var validProxies = []string{
	"127.0.0.1:8080",
	"::127.0.0.1:8080", // empty user and pass is valid
	"google.com:8080",
	"user:pass@127.0.0.1:9022",
	":@127.0.0.1:9022",
	"user:pass:localhost:1234",
	"::localhost:",
}

// Need to test FromReader too
func TestProxy(t *testing.T) {
	var uninitialized *Proxy

	assert.Equal(t, uninitialized.String(), "", "nil Proxy string not empty")

	assert.Error(t, uninitialized.Validate(), "nil proxy is not valid")
}

func TestFromString(t *testing.T) {
	t.Run("WithInvalidProxies", func(t *testing.T) {
		for _, proxy := range invalidProxies {
			_, err := FromString(proxy.url)
			assert.Errorf(t, err, proxy.reason)
		}
	})
	t.Run("WithValidProxies", func(t *testing.T) {
		for _, proxy := range validProxies {
			_, err := FromString(proxy)
			assert.NoErrorf(t, err, "valid proxy %s should have err = nil", proxy)
		}
	})

}

func TestFromReader(t *testing.T) {
	t.Run("WithValidInput", func(t *testing.T) {
		proxiesStr := "user:pass@host:8080\nhost:8080"
		reader := strings.NewReader(proxiesStr)

		proxies, err := FromReader(reader)

		assert.NoError(t, err)
		assert.Len(t, proxies, 2)
		assert.Equal(t, "http://user:pass@host:8080", proxies[0].String())
		assert.Equal(t, "http://host:8080", proxies[1].String())
	})

	t.Run("WithInvalidInput", func(t *testing.T) {
		proxiesStr := "user:pass@host:8080\nhost:8080\ninvalid"
		reader := strings.NewReader(proxiesStr)

		_, err := FromReader(reader)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid proxy")
	})

	t.Run("WithEmptyInput", func(t *testing.T) {
		reader := bytes.NewReader([]byte{})

		proxies, err := FromReader(reader)
		assert.NoError(t, err)
		assert.Len(t, proxies, 0)
	})
}

package notify

import (
	"testing"

	"github.com/rodjunger/nkmonitor"
	"github.com/stretchr/testify/assert"
)

func TestNewDiscordNotifyer(t *testing.T) {
	tests := []struct {
		name       string
		webhookURL string
		wantErr    bool
	}{
		{
			name:       "empty webhook URL",
			webhookURL: "",
			wantErr:    true,
		},
		{
			name:       "valid webhook URL",
			webhookURL: "https://discordapp.com/api/webhooks/123456/abcdefghijklmnopqrstuvwxyz",
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewDiscordNotifyer(tt.webhookURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDiscordNotifyer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && got == nil {
				t.Errorf("NewDiscordNotifyer() = nil, want instance")
			}
		})
	}
}

func TestSizeToString(t *testing.T) {
	t.Run("WithAvailableSize", func(t *testing.T) {
		size := &nkmonitor.SizeInfo{
			Description: "8",
			Sku:         "12345",
			IsAvailable: true,
			Restocked:   true,
		}

		result := sizeToString(size)
		expected := "8 12345 ✓"

		assert.Equal(t, result, expected)
	})

	t.Run("WithUnavailableSize", func(t *testing.T) {
		size := &nkmonitor.SizeInfo{
			Description: "9",
			Sku:         "54321",
			IsAvailable: false,
			Restocked:   false,
		}

		result := sizeToString(size)
		expected := "9 54321 x"

		assert.Equal(t, result, expected)
	})
}

func TestNotifyu(t *testing.T) {
	t.Run("WithUnknownWebhook", func(t *testing.T) {
		webhookUrl := "https://discordapp.com/api/webhooks/1234567890/abcdefghijklmnopqrstuvwxyz"
		notifyer, err := NewDiscordNotifyer(webhookUrl)
		assert.NoError(t, err, "Failed to create DiscordNotifyer: %v", err)

		info := nkmonitor.RestockInfo{
			Name:    "Shoe",
			Path:    "/tenis/test.html",
			Code:    "ABC123",
			Price:   "R$ 100,00",
			Sizes:   []*nkmonitor.SizeInfo{},
			Picture: "https://example.com/picture.jpg",
		}

		err = notifyer.Notify(info)
		assert.Error(t, err, "Expected an error but got nil")
		assert.EqualError(t, err, "Status: 404 Not Found, Body: {\"message\": \"Unknown Webhook\", \"code\": 10015}", "Expected error %q but got %q", "Status: 404 Not Found, Body: {\"message\": \"Unknown Webhook\", \"code\": 10015}", err.Error())
	})

	t.Run("WithNilInstance", func(t *testing.T) {
		var notifyer *DiscordNotifyer
		err := notifyer.Notify(nkmonitor.RestockInfo{})
		assert.Error(t, err, "Expected an error but got nil")
		assert.EqualError(t, err, "nil instance", "Expected error %q but got %q", "nil instance", err.Error())
	})

	t.Run("WithInvalidWebhookUrl", func(t *testing.T) {
		webhookUrl := "invalid"
		notifyer, err := NewDiscordNotifyer(webhookUrl)
		assert.Error(t, err, "Expected an error but got nil")
		assert.EqualError(t, err, "invalid webhook", "Expected error %q but got %q", "invalid webhook", err.Error())
		assert.Nil(t, notifyer, "Expected a nil notifyer but got a non-nil value")
	})
}

package notifications

import (
	"encoding/json"
	"fmt"
	"os"

	webpush "github.com/SherClockHolmes/webpush-go"
	"go.uber.org/zap"
)

// VAPIDKeys holds the VAPID public/private key pair for Web Push.
type VAPIDKeys struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}

// GenerateVAPIDKeys creates a new VAPID key pair.
func GenerateVAPIDKeys() (*VAPIDKeys, error) {
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return nil, err
	}
	return &VAPIDKeys{Public: pub, Private: priv}, nil
}

// LoadOrGenerateVAPIDKeys reads VAPID keys from path. If the file does not
// exist, a new pair is generated and written to path.
func LoadOrGenerateVAPIDKeys(path string, logger *zap.Logger) (*VAPIDKeys, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		var k VAPIDKeys
		if jsonErr := json.Unmarshal(data, &k); jsonErr == nil && k.Public != "" {
			return &k, nil
		}
	}
	// Generate new keys.
	k, err := GenerateVAPIDKeys()
	if err != nil {
		return nil, fmt.Errorf("generate VAPID keys: %w", err)
	}
	data, err = json.MarshalIndent(k, "", "  ")
	if err != nil {
		return nil, err
	}
	if writeErr := os.WriteFile(path, data, 0600); writeErr != nil {
		logger.Warn("Failed to persist VAPID keys", zap.String("path", path), zap.Error(writeErr))
	} else {
		logger.Info("Generated VAPID keys", zap.String("path", path))
	}
	return k, nil
}

// PushPayload is the JSON body sent inside a push notification.
type PushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Tag   string `json:"tag,omitempty"`
	URL   string `json:"url,omitempty"`
}

// WebPushSub is a normalised push subscription (endpoint + p256dh + auth).
type WebPushSub struct {
	Endpoint string
	P256DH   string
	Auth     string
}

// sendWebPush delivers a single push notification.
// Returns a non-nil error on failure; a 410 (Gone) response means the
// subscription has expired and should be removed from storage.
func sendWebPush(sub WebPushSub, payload PushPayload, keys VAPIDKeys) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	s := &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256DH,
			Auth:   sub.Auth,
		},
	}
	resp, err := webpush.SendNotification(data, s, &webpush.Options{
		VAPIDPublicKey:  keys.Public,
		VAPIDPrivateKey: keys.Private,
		TTL:             60,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 410 {
		return fmt.Errorf("subscription expired (410): %s", sub.Endpoint)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("push service returned HTTP %d for %s", resp.StatusCode, sub.Endpoint)
	}
	return nil
}

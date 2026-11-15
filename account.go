package wsw

import (
	"context"
	"fmt"
	"net/http"
)

// Account is whose API key this is: the answer of [InstantSnapService.Account] and
// [AutoScheduleService.Account]. It never includes the key itself.
type Account struct {
	// KeyName is the name given to the key in the dashboard (Developer Tools, API Keys), or nil.
	KeyName *string `json:"key_name"`
	// KeyPrefix is the key's public prefix, as the dashboard shows it. It is not enough to use
	// the key.
	KeyPrefix string `json:"key_prefix"`
	// Product is the product the key belongs to.
	Product Product `json:"product"`
	// Email is the member who created the key, or nil when they are no longer a member.
	Email *string `json:"email"`
}

// Account answers whose Instant Snap key the client holds: its name and prefix, its product and
// the member who created it. It costs no capture, so it is also a cheap way to check a key works.
func (s *InstantSnapService) Account(ctx context.Context) (*Account, error) {
	key, err := s.key()
	if err != nil {
		return nil, err
	}
	return s.client.account(ctx, s.client.instantSnapBaseURL+"/instant-snap/v1/account", key)
}

// Account answers whose Auto Schedule key the client holds. See [InstantSnapService.Account].
func (s *AutoScheduleService) Account(ctx context.Context) (*Account, error) {
	if s.client.autoScheduleKey == "" {
		return nil, fmt.Errorf("%w: the Auto Schedule key is not set; use wsw.WithAutoScheduleKey or set %s", ErrMissingKey, EnvAutoScheduleKey)
	}
	return s.client.account(ctx, s.client.autoScheduleBaseURL+"/auto-schedule/v1/account", s.client.autoScheduleKey)
}

func (c *Client) account(ctx context.Context, url, key string) (*Account, error) {
	var a Account
	_, err := c.sendJSON(ctx, &request{
		method:  http.MethodGet,
		url:     url,
		apiKey:  key,
		timeout: c.timeout,
	}, &a)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

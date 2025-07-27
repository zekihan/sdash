package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zekihan/sdash/config"
)

func TestIsExpired(t *testing.T) {
	now := time.Now()

	// Test case: Cookie is not expired
	cookie := &Cookie{
		ExpireTime: now.Add(1 * time.Hour),
	}
	assert.False(t, cookie.IsExpired(), "Expected cookie to not be expired")

	// Test case: Cookie is expired
	cookie = &Cookie{
		ExpireTime: now.Add(-1 * time.Hour),
	}
	assert.True(t, cookie.IsExpired(), "Expected cookie to be expired")
}

func TestNewCookie(t *testing.T) {
	// Mock configuration
	config.GetConfig().WebConfig.SessionTimeout = 2 * time.Hour

	cookie := NewCookie()

	assert.NotNil(t, cookie, "Expected cookie to be created")
	assert.NotEmpty(t, cookie.ID, "Expected cookie ID to be set")
	assert.False(t, cookie.Authenticated, "Expected cookie to be unauthenticated")
	assert.WithinDuration(t, time.Now(), cookie.Created, 1*time.Second, "Expected cookie creation time to be close to now")
	assert.WithinDuration(t, time.Now().Add(2*time.Hour), cookie.ExpireTime, 1*time.Second, "Expected cookie expiration time to match session timeout")
}

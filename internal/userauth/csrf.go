package userauth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type csrfBuffer struct {
	tokens []csrfToken
	mu     sync.Mutex
}

type csrfToken struct {
	value     string
	expiresAt time.Time
}

func (b *csrfBuffer) generate() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	tokenValue := hex.EncodeToString(bytes)

	b.mu.Lock()
	defer b.mu.Unlock()

	b.tokens = append(b.tokens, csrfToken{
		value:     tokenValue,
		expiresAt: time.Now().Add(15 * time.Minute),
	})
	return tokenValue
}

func (b *csrfBuffer) check(tokenValue string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	isValid := false
	retained := make([]csrfToken, 0, 8)
	for _, token := range b.tokens {
		// If the token has expired, purge it
		hasExpired := token.expiresAt.Before(time.Now())
		if hasExpired {
			continue
		}

		// If the matches our desired value, then we can validate it and drop it from
		// the buffer since it's been used
		if token.value == tokenValue {
			isValid = true
			continue
		}

		// All other not-yet-expired CSRF tokens should be retained
		retained = append(retained, token)
	}
	b.tokens = retained

	return isValid
}

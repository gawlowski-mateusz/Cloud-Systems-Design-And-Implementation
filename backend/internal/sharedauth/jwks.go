package sharedauth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwks struct {
	Keys []jwk `json:"keys"`
}

type KeySet struct {
	url   string
	mu    sync.RWMutex
	keys  map[string]*rsa.PublicKey
	fresh time.Time
}

func NewKeySet(region, userPoolID string) *KeySet {
	url := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json", region, userPoolID)
	return &KeySet{url: url, keys: map[string]*rsa.PublicKey{}}
}

func (k *KeySet) Refresh() error {
	req, err := http.NewRequest(http.MethodGet, k.url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jwks endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var set jwks
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return fmt.Errorf("failed to parse jwks: %w", err)
	}

	parsed := map[string]*rsa.PublicKey{}
	for _, key := range set.Keys {
		pub, err := jwkToRSA(key)
		if err != nil {
			continue
		}
		parsed[key.Kid] = pub
	}

	k.mu.Lock()
	k.keys = parsed
	k.fresh = time.Now()
	k.mu.Unlock()
	return nil
}

func (k *KeySet) Key(kid string) (*rsa.PublicKey, error) {
	k.mu.RLock()
	pub, ok := k.keys[kid]
	stale := time.Since(k.fresh) > time.Hour
	k.mu.RUnlock()

	if ok && !stale {
		return pub, nil
	}

	if err := k.Refresh(); err != nil {
		if ok {
			return pub, nil
		}
		return nil, err
	}

	k.mu.RLock()
	defer k.mu.RUnlock()
	pub, ok = k.keys[kid]
	if !ok {
		return nil, errors.New("kid not found in jwks")
	}
	return pub, nil
}

func jwkToRSA(key jwk) (*rsa.PublicKey, error) {
	if key.Kty != "RSA" {
		return nil, errors.New("unsupported kty")
	}
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("invalid modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("invalid exponent: %w", err)
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}

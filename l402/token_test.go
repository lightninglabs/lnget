package l402

import (
	"crypto/rand"
	"testing"

	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/stretchr/testify/require"
	"gopkg.in/macaroon.v2"
)

// makeMacaroon creates a test macaroon with a random nonce.
func makeMacaroon(t *testing.T) *macaroon.Macaroon {
	t.Helper()

	var nonce [32]byte

	_, err := rand.Read(nonce[:])
	require.NoError(t, err)

	mac, err := macaroon.New(
		nonce[:], nonce[:], "test", macaroon.LatestVersion,
	)
	require.NoError(t, err)

	return mac
}

// TestNewTokenFromChallenge verifies that NewTokenFromChallenge creates a
// Token with the base macaroon properly populated (not nil), which is
// critical for subsequent serialization via aperture's StoreToken.
func TestNewTokenFromChallenge(t *testing.T) {
	t.Parallel()

	mac := makeMacaroon(t)
	macBytes, err := mac.MarshalBinary()
	require.NoError(t, err)

	var paymentHash [32]byte

	_, err = rand.Read(paymentHash[:])
	require.NoError(t, err)

	token, err := NewTokenFromChallenge(macBytes, paymentHash)
	require.NoError(t, err)
	require.NotNil(t, token)

	// The payment hash should match what we provided.
	require.Equal(t, lntypes.Hash(paymentHash), token.PaymentHash)

	// The base macaroon must not be nil — this is the field that
	// caused the nil pointer panic before the fix.
	baseMac := token.BaseMacaroon()
	require.NotNil(t, baseMac, "BaseMacaroon() must not return nil")

	// The macaroon data should round-trip correctly.
	roundTripBytes, err := baseMac.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, macBytes, roundTripBytes,
		"macaroon bytes should survive round-trip")

	// A newly created token should be pending (zero preimage).
	require.True(t, IsPending(token),
		"new token should be pending (zero preimage)")
}

// TestNewTokenFromChallenge_InvalidMacaroon verifies that invalid macaroon
// bytes are rejected.
func TestNewTokenFromChallenge_InvalidMacaroon(t *testing.T) {
	t.Parallel()

	invalidMac := []byte("not-a-valid-macaroon")

	var paymentHash [32]byte

	_, err := NewTokenFromChallenge(invalidMac, paymentHash)
	require.Error(t, err, "should reject invalid macaroon bytes")
}

// TestNewTokenFromChallenge_StorePending verifies that a token created by
// NewTokenFromChallenge can be stored via the FileStore without panicking.
// This is the exact code path that triggered the original nil pointer
// dereference.
func TestNewTokenFromChallenge_StorePending(t *testing.T) {
	t.Parallel()

	mac := makeMacaroon(t)
	macBytes, err := mac.MarshalBinary()
	require.NoError(t, err)

	var paymentHash [32]byte

	_, err = rand.Read(paymentHash[:])
	require.NoError(t, err)

	token, err := NewTokenFromChallenge(macBytes, paymentHash)
	require.NoError(t, err)

	// Create a FileStore and store the pending token — this is what
	// crashed with the nil baseMac before the fix.
	store, err := NewFileStore(t.TempDir())
	require.NoError(t, err)

	err = store.StorePending("test.example.com", token)
	require.NoError(t, err, "StorePending must not panic or error")

	// Verify we can read it back.
	retrieved, err := store.GetToken("test.example.com")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	require.Equal(t, token.PaymentHash, retrieved.PaymentHash)
}

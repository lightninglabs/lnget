// Package l402 provides per-domain L402 token management, wrapping and
// extending the aperture/l402 library for lnget's HTTP client use case.
package l402

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lightninglabs/aperture/l402"
	"github.com/lightningnetwork/lnd/lntypes"
)

// Token is an alias for the aperture l402.Token type.
type Token = l402.Token

// tokenByteOrder is the byte order used by aperture's token serialization
// format. Must match aperture/l402/identifier.go's byteOrder.
var tokenByteOrder = binary.BigEndian

// NewTokenFromChallenge creates a new pending Token with the base macaroon
// properly initialized from the raw macaroon bytes and payment hash
// extracted from an L402 challenge.
//
// Because aperture's Token stores the base macaroon in an unexported field
// (baseMac) with no public constructor, we serialize the token in aperture's
// binary wire format, write it to a temporary FileStore, and read it back.
// This round-trip through aperture's own deserialization ensures the
// baseMac field is properly populated.
func NewTokenFromChallenge(macBytes []byte,
	paymentHash [32]byte) (*Token, error) {
	// Build the token in aperture's binary serialization format:
	//   [4]  macLen      uint32
	//   [N]  macBytes    []byte
	//   [32] paymentHash lntypes.Hash
	//   [32] preimage    lntypes.Preimage (zeroed for pending)
	//   [8]  amountPaid  uint64 (MilliSatoshi)
	//   [8]  routingFee  uint64 (MilliSatoshi)
	//   [8]  timeCreated int64 (UnixNano)
	var buf bytes.Buffer

	macLen := uint32(len(macBytes))

	err := binary.Write(&buf, tokenByteOrder, macLen)
	if err != nil {
		return nil, fmt.Errorf("failed to write mac length: %w", err)
	}

	err = binary.Write(&buf, tokenByteOrder, macBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to write mac bytes: %w", err)
	}

	err = binary.Write(&buf, tokenByteOrder, paymentHash)
	if err != nil {
		return nil, fmt.Errorf("failed to write payment hash: %w",
			err)
	}

	// Zero preimage indicates a pending token.
	var zeroPreimage lntypes.Preimage

	err = binary.Write(&buf, tokenByteOrder, zeroPreimage)
	if err != nil {
		return nil, fmt.Errorf("failed to write preimage: %w", err)
	}

	// Zero amounts for a newly created token.
	err = binary.Write(&buf, tokenByteOrder, uint64(0))
	if err != nil {
		return nil, fmt.Errorf("failed to write amount: %w", err)
	}

	err = binary.Write(&buf, tokenByteOrder, uint64(0))
	if err != nil {
		return nil, fmt.Errorf("failed to write routing fee: %w", err)
	}

	timeNano := time.Now().UnixNano()

	err = binary.Write(&buf, tokenByteOrder, timeNano)
	if err != nil {
		return nil, fmt.Errorf("failed to write timestamp: %w", err)
	}

	// Round-trip through aperture's FileStore to get a Token with
	// baseMac properly set via aperture's deserializeToken.
	tmpDir, err := os.MkdirTemp("", "lnget-token-roundtrip-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Write as a pending token file (aperture uses "l402.token.pending").
	pendingFile := filepath.Join(tmpDir, "l402.token.pending")

	err = os.WriteFile(pendingFile, buf.Bytes(), 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to write temp token: %w", err)
	}

	// Read back via aperture's FileStore which deserializes with baseMac.
	store, err := l402.NewFileStore(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp store: %w", err)
	}

	token, err := store.CurrentToken()
	if err != nil {
		return nil, fmt.Errorf("failed to read back token: %w", err)
	}

	return token, nil
}

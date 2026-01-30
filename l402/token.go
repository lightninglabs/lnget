// Package l402 provides per-domain L402 token management, wrapping and
// extending the aperture/l402 library for lnget's HTTP client use case.
package l402

import (
	"github.com/lightninglabs/aperture/l402"
)

// Token is an alias for the aperture l402.Token type.
type Token = l402.Token

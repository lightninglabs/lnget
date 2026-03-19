package mpp

import (
	"github.com/btcsuite/btclog/v2"
	"github.com/lightninglabs/lnget/build"
)

// log is the package-level logger for the mpp subsystem.
var log btclog.Logger

func init() {
	log = build.RegisterSubLogger("MPP", func(l btclog.Logger) {
		log = l
	})
}

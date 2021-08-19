// Copyright 2015 The go-AVNereum Authors
// This file is part of the go-AVNereum library.
//
// The go-AVNereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-AVNereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-AVNereum library. If not, see <http://www.gnu.org/licenses/>.

// Contains the metrics collected by the downloader.

package downloader

import (
	"github.com/AVNereum/go-AVNereum/metrics"
)

var (
	headerInMeter      = metrics.NewRegisteredMeter("AVN/downloader/headers/in", nil)
	headerReqTimer     = metrics.NewRegisteredTimer("AVN/downloader/headers/req", nil)
	headerDropMeter    = metrics.NewRegisteredMeter("AVN/downloader/headers/drop", nil)
	headerTimeoutMeter = metrics.NewRegisteredMeter("AVN/downloader/headers/timeout", nil)

	bodyInMeter      = metrics.NewRegisteredMeter("AVN/downloader/bodies/in", nil)
	bodyReqTimer     = metrics.NewRegisteredTimer("AVN/downloader/bodies/req", nil)
	bodyDropMeter    = metrics.NewRegisteredMeter("AVN/downloader/bodies/drop", nil)
	bodyTimeoutMeter = metrics.NewRegisteredMeter("AVN/downloader/bodies/timeout", nil)

	receiptInMeter      = metrics.NewRegisteredMeter("AVN/downloader/receipts/in", nil)
	receiptReqTimer     = metrics.NewRegisteredTimer("AVN/downloader/receipts/req", nil)
	receiptDropMeter    = metrics.NewRegisteredMeter("AVN/downloader/receipts/drop", nil)
	receiptTimeoutMeter = metrics.NewRegisteredMeter("AVN/downloader/receipts/timeout", nil)

	stateInMeter   = metrics.NewRegisteredMeter("AVN/downloader/states/in", nil)
	stateDropMeter = metrics.NewRegisteredMeter("AVN/downloader/states/drop", nil)

	throttleCounter = metrics.NewRegisteredCounter("AVN/downloader/throttle", nil)
)

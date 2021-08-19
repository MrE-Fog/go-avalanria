// Copyright 2021 The go-avalanria Authors
// This file is part of the go-avalanria library.
//
// The go-avalanria library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-avalanria library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-avalanria library. If not, see <http://www.gnu.org/licenses/>.

package server

import (
	"github.com/avalanria/go-avalanria/metrics"
)

var (
	totalConnectedGauge = metrics.NewRegisteredGauge("vflux/server/totalConnected", nil)

	clientConnectedMeter    = metrics.NewRegisteredMeter("vflux/server/clientEvent/connected", nil)
	clientActivatedMeter    = metrics.NewRegisteredMeter("vflux/server/clientEvent/activated", nil)
	clientDeactivatedMeter  = metrics.NewRegisteredMeter("vflux/server/clientEvent/deactivated", nil)
	clientDisconnectedMeter = metrics.NewRegisteredMeter("vflux/server/clientEvent/disconnected", nil)

	capacityQueryZeroMeter    = metrics.NewRegisteredMeter("vflux/server/capQueryZero", nil)
	capacityQueryNonZeroMeter = metrics.NewRegisteredMeter("vflux/server/capQueryNonZero", nil)
)

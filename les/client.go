// Copyright 2016 The go-AVNereum Authors
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

// Package les implements the Light Avalanria Subprotocol.
package les

import (
	"fmt"
	"time"

	"github.com/AVNereum/go-AVNereum/accounts"
	"github.com/AVNereum/go-AVNereum/common"
	"github.com/AVNereum/go-AVNereum/common/hexutil"
	"github.com/AVNereum/go-AVNereum/common/mclock"
	"github.com/AVNereum/go-AVNereum/consensus"
	"github.com/AVNereum/go-AVNereum/core"
	"github.com/AVNereum/go-AVNereum/core/bloombits"
	"github.com/AVNereum/go-AVNereum/core/rawdb"
	"github.com/AVNereum/go-AVNereum/core/types"
	"github.com/AVNereum/go-AVNereum/AVN/downloader"
	"github.com/AVNereum/go-AVNereum/AVN/AVNconfig"
	"github.com/AVNereum/go-AVNereum/AVN/filters"
	"github.com/AVNereum/go-AVNereum/AVN/gasprice"
	"github.com/AVNereum/go-AVNereum/event"
	"github.com/AVNereum/go-AVNereum/internal/AVNapi"
	"github.com/AVNereum/go-AVNereum/les/vflux"
	vfc "github.com/AVNereum/go-AVNereum/les/vflux/client"
	"github.com/AVNereum/go-AVNereum/light"
	"github.com/AVNereum/go-AVNereum/log"
	"github.com/AVNereum/go-AVNereum/node"
	"github.com/AVNereum/go-AVNereum/p2p"
	"github.com/AVNereum/go-AVNereum/p2p/enode"
	"github.com/AVNereum/go-AVNereum/p2p/enr"
	"github.com/AVNereum/go-AVNereum/params"
	"github.com/AVNereum/go-AVNereum/rlp"
	"github.com/AVNereum/go-AVNereum/rpc"
)

type LightAvalanria struct {
	lesCommons

	peers              *serverPeerSet
	reqDist            *requestDistributor
	retriever          *retrieveManager
	odr                *LesOdr
	relay              *lesTxRelay
	handler            *clientHandler
	txPool             *light.TxPool
	blockchain         *light.LightChain
	serverPool         *vfc.ServerPool
	serverPoolIterator enode.Iterator
	pruner             *pruner

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	ApiBackend     *LesApiBackend
	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager
	netRPCService  *AVNapi.PublicNetAPI

	p2pServer  *p2p.Server
	p2pConfig  *p2p.Config
	udpEnabled bool
}

// New creates an instance of the light client.
func New(stack *node.Node, config *AVNconfig.Config) (*LightAvalanria, error) {
	chainDb, err := stack.OpenDatabase("lightchaindata", config.DatabaseCache, config.DatabaseHandles, "AVN/db/chaindata/", false)
	if err != nil {
		return nil, err
	}
	lesDb, err := stack.OpenDatabase("les.client", 0, 0, "AVN/db/lesclient/", false)
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlockWithOverride(chainDb, config.Genesis, config.OverrideLondon)
	if _, isCompat := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !isCompat {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	peers := newServerPeerSet()
	lAVN := &LightAvalanria{
		lesCommons: lesCommons{
			genesis:     genesisHash,
			config:      config,
			chainConfig: chainConfig,
			iConfig:     light.DefaultClientIndexerConfig,
			chainDb:     chainDb,
			lesDb:       lesDb,
			closeCh:     make(chan struct{}),
		},
		peers:          peers,
		eventMux:       stack.EventMux(),
		reqDist:        newRequestDistributor(peers, &mclock.System{}),
		accountManager: stack.AccountManager(),
		engine:         AVNconfig.CreateConsensusEngine(stack, chainConfig, &config.Ethash, nil, false, chainDb),
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   core.NewBloomIndexer(chainDb, params.BloomBitsBlocksClient, params.HelperTrieConfirmations),
		p2pServer:      stack.Server(),
		p2pConfig:      &stack.Config().P2P,
		udpEnabled:     stack.Config().P2P.DiscoveryV5,
	}

	var prenegQuery vfc.QueryFunc
	if lAVN.udpEnabled {
		prenegQuery = lAVN.prenegQuery
	}
	lAVN.serverPool, lAVN.serverPoolIterator = vfc.NewServerPool(lesDb, []byte("serverpool:"), time.Second, prenegQuery, &mclock.System{}, config.UltraLightServers, requestList)
	lAVN.serverPool.AddMetrics(suggestedTimeoutGauge, totalValueGauge, serverSelectableGauge, serverConnectedGauge, sessionValueMeter, serverDialedMeter)

	lAVN.retriever = newRetrieveManager(peers, lAVN.reqDist, lAVN.serverPool.GetTimeout)
	lAVN.relay = newLesTxRelay(peers, lAVN.retriever)

	lAVN.odr = NewLesOdr(chainDb, light.DefaultClientIndexerConfig, lAVN.peers, lAVN.retriever)
	lAVN.chtIndexer = light.NewChtIndexer(chainDb, lAVN.odr, params.CHTFrequency, params.HelperTrieConfirmations, config.LightNoPrune)
	lAVN.bloomTrieIndexer = light.NewBloomTrieIndexer(chainDb, lAVN.odr, params.BloomBitsBlocksClient, params.BloomTrieFrequency, config.LightNoPrune)
	lAVN.odr.SetIndexers(lAVN.chtIndexer, lAVN.bloomTrieIndexer, lAVN.bloomIndexer)

	checkpoint := config.Checkpoint
	if checkpoint == nil {
		checkpoint = params.TrustedCheckpoints[genesisHash]
	}
	// Note: NewLightChain adds the trusted checkpoint so it needs an ODR with
	// indexers already set but not started yet
	if lAVN.blockchain, err = light.NewLightChain(lAVN.odr, lAVN.chainConfig, lAVN.engine, checkpoint); err != nil {
		return nil, err
	}
	lAVN.chainReader = lAVN.blockchain
	lAVN.txPool = light.NewTxPool(lAVN.chainConfig, lAVN.blockchain, lAVN.relay)

	// Set up checkpoint oracle.
	lAVN.oracle = lAVN.setupOracle(stack, genesisHash, config)

	// Note: AddChildIndexer starts the update process for the child
	lAVN.bloomIndexer.AddChildIndexer(lAVN.bloomTrieIndexer)
	lAVN.chtIndexer.Start(lAVN.blockchain)
	lAVN.bloomIndexer.Start(lAVN.blockchain)

	// Start a light chain pruner to delete useless historical data.
	lAVN.pruner = newPruner(chainDb, lAVN.chtIndexer, lAVN.bloomTrieIndexer)

	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		lAVN.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}

	lAVN.ApiBackend = &LesApiBackend{stack.Config().ExtRPCEnabled(), stack.Config().AllowUnprotectedTxs, lAVN, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.Miner.GasPrice
	}
	lAVN.ApiBackend.gpo = gasprice.NewOracle(lAVN.ApiBackend, gpoParams)

	lAVN.handler = newClientHandler(config.UltraLightServers, config.UltraLightFraction, checkpoint, lAVN)
	if lAVN.handler.ulc != nil {
		log.Warn("Ultra light client is enabled", "trustedNodes", len(lAVN.handler.ulc.keys), "minTrustedFraction", lAVN.handler.ulc.fraction)
		lAVN.blockchain.DisableCheckFreq()
	}

	lAVN.netRPCService = AVNapi.NewPublicNetAPI(lAVN.p2pServer, lAVN.config.NetworkId)

	// Register the backend on the node
	stack.RegisterAPIs(lAVN.APIs())
	stack.RegisterProtocols(lAVN.Protocols())
	stack.RegisterLifecycle(lAVN)

	// Check for unclean shutdown
	if uncleanShutdowns, discards, err := rawdb.PushUncleanShutdownMarker(chainDb); err != nil {
		log.Error("Could not update unclean-shutdown-marker list", "error", err)
	} else {
		if discards > 0 {
			log.Warn("Old unclean shutdowns found", "count", discards)
		}
		for _, tstamp := range uncleanShutdowns {
			t := time.Unix(int64(tstamp), 0)
			log.Warn("Unclean shutdown detected", "booted", t,
				"age", common.PrettyAge(t))
		}
	}
	return lAVN, nil
}

// VfluxRequest sends a batch of requests to the given node through discv5 UDP TalkRequest and returns the responses
func (s *LightAvalanria) VfluxRequest(n *enode.Node, reqs vflux.Requests) vflux.Replies {
	if !s.udpEnabled {
		return nil
	}
	reqsEnc, _ := rlp.EncodeToBytes(&reqs)
	repliesEnc, _ := s.p2pServer.DiscV5.TalkRequest(s.serverPool.DialNode(n), "vfx", reqsEnc)
	var replies vflux.Replies
	if len(repliesEnc) == 0 || rlp.DecodeBytes(repliesEnc, &replies) != nil {
		return nil
	}
	return replies
}

// vfxVersion returns the version number of the "les" service subdomain of the vflux UDP
// service, as advertised in the ENR record
func (s *LightAvalanria) vfxVersion(n *enode.Node) uint {
	if n.Seq() == 0 {
		var err error
		if !s.udpEnabled {
			return 0
		}
		if n, err = s.p2pServer.DiscV5.RequestENR(n); n != nil && err == nil && n.Seq() != 0 {
			s.serverPool.Persist(n)
		} else {
			return 0
		}
	}

	var les []rlp.RawValue
	if err := n.Load(enr.WithEntry("les", &les)); err != nil || len(les) < 1 {
		return 0
	}
	var version uint
	rlp.DecodeBytes(les[0], &version) // Ignore additional fields (for forward compatibility).
	return version
}

// prenegQuery sends a capacity query to the given server node to determine whAVNer
// a connection slot is immediately available
func (s *LightAvalanria) prenegQuery(n *enode.Node) int {
	if s.vfxVersion(n) < 1 {
		// UDP query not supported, always try TCP connection
		return 1
	}

	var requests vflux.Requests
	requests.Add("les", vflux.CapacityQueryName, vflux.CapacityQueryReq{
		Bias:      180,
		AddTokens: []vflux.IntOrInf{{}},
	})
	replies := s.VfluxRequest(n, requests)
	var cqr vflux.CapacityQueryReply
	if replies.Get(0, &cqr) != nil || len(cqr) != 1 { // Note: Get returns an error if replies is nil
		return -1
	}
	if cqr[0] > 0 {
		return 1
	}
	return 0
}

type LightDummyAPI struct{}

// Etherbase is the address that mining rewards will be send to
func (s *LightDummyAPI) Etherbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("mining is not supported in light mode")
}

// Coinbase is the address that mining rewards will be send to (alias for Etherbase)
func (s *LightDummyAPI) Coinbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("mining is not supported in light mode")
}

// Hashrate returns the POW hashrate
func (s *LightDummyAPI) Hashrate() hexutil.Uint {
	return 0
}

// Mining returns an indication if this node is currently mining.
func (s *LightDummyAPI) Mining() bool {
	return false
}

// APIs returns the collection of RPC services the AVNereum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *LightAvalanria) APIs() []rpc.API {
	apis := AVNapi.GetAPIs(s.ApiBackend)
	apis = append(apis, s.engine.APIs(s.BlockChain().HeaderChain())...)
	return append(apis, []rpc.API{
		{
			Namespace: "AVN",
			Version:   "1.0",
			Service:   &LightDummyAPI{},
			Public:    true,
		}, {
			Namespace: "AVN",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.handler.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "AVN",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.ApiBackend, true, 5*time.Minute),
			Public:    true,
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		}, {
			Namespace: "les",
			Version:   "1.0",
			Service:   NewPrivateLightAPI(&s.lesCommons),
			Public:    false,
		}, {
			Namespace: "vflux",
			Version:   "1.0",
			Service:   s.serverPool.API(),
			Public:    false,
		},
	}...)
}

func (s *LightAvalanria) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *LightAvalanria) BlockChain() *light.LightChain      { return s.blockchain }
func (s *LightAvalanria) TxPool() *light.TxPool              { return s.txPool }
func (s *LightAvalanria) Engine() consensus.Engine           { return s.engine }
func (s *LightAvalanria) LesVersion() int                    { return int(ClientProtocolVersions[0]) }
func (s *LightAvalanria) Downloader() *downloader.Downloader { return s.handler.downloader }
func (s *LightAvalanria) EventMux() *event.TypeMux           { return s.eventMux }

// Protocols returns all the currently configured network protocols to start.
func (s *LightAvalanria) Protocols() []p2p.Protocol {
	return s.makeProtocols(ClientProtocolVersions, s.handler.runPeer, func(id enode.ID) interface{} {
		if p := s.peers.peer(id.String()); p != nil {
			return p.Info()
		}
		return nil
	}, s.serverPoolIterator)
}

// Start implements node.Lifecycle, starting all internal goroutines needed by the
// light AVNereum protocol implementation.
func (s *LightAvalanria) Start() error {
	log.Warn("Light client mode is an experimental feature")

	if s.udpEnabled && s.p2pServer.DiscV5 == nil {
		s.udpEnabled = false
		log.Error("Discovery v5 is not initialized")
	}
	discovery, err := s.setupDiscovery()
	if err != nil {
		return err
	}
	s.serverPool.AddSource(discovery)
	s.serverPool.Start()
	// Start bloom request workers.
	s.wg.Add(bloomServiceThreads)
	s.startBloomHandlers(params.BloomBitsBlocksClient)
	s.handler.start()

	return nil
}

// Stop implements node.Lifecycle, terminating all internal goroutines used by the
// Avalanria protocol.
func (s *LightAvalanria) Stop() error {
	close(s.closeCh)
	s.serverPool.Stop()
	s.peers.close()
	s.reqDist.close()
	s.odr.Stop()
	s.relay.Stop()
	s.bloomIndexer.Close()
	s.chtIndexer.Close()
	s.blockchain.Stop()
	s.handler.stop()
	s.txPool.Stop()
	s.engine.Close()
	s.pruner.close()
	s.eventMux.Stop()
	rawdb.PopUncleanShutdownMarker(s.chainDb)
	s.chainDb.Close()
	s.lesDb.Close()
	s.wg.Wait()
	log.Info("Light AVNereum stopped")
	return nil
}

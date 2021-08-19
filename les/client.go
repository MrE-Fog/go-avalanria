// Copyright 2016 The go-avalanria Authors
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

// Package les implements the Light Avalanria Subprotocol.
package les

import (
	"fmt"
	"time"

	"github.com/avalanria/go-avalanria/accounts"
	"github.com/avalanria/go-avalanria/common"
	"github.com/avalanria/go-avalanria/common/hexutil"
	"github.com/avalanria/go-avalanria/common/mclock"
	"github.com/avalanria/go-avalanria/consensus"
	"github.com/avalanria/go-avalanria/core"
	"github.com/avalanria/go-avalanria/core/bloombits"
	"github.com/avalanria/go-avalanria/core/rawdb"
	"github.com/avalanria/go-avalanria/core/types"
	"github.com/avalanria/go-avalanria/avn/downloader"
	"github.com/avalanria/go-avalanria/avn/avnconfig"
	"github.com/avalanria/go-avalanria/avn/filters"
	"github.com/avalanria/go-avalanria/avn/gasprice"
	"github.com/avalanria/go-avalanria/event"
	"github.com/avalanria/go-avalanria/internal/avnapi"
	"github.com/avalanria/go-avalanria/les/vflux"
	vfc "github.com/avalanria/go-avalanria/les/vflux/client"
	"github.com/avalanria/go-avalanria/light"
	"github.com/avalanria/go-avalanria/log"
	"github.com/avalanria/go-avalanria/node"
	"github.com/avalanria/go-avalanria/p2p"
	"github.com/avalanria/go-avalanria/p2p/enode"
	"github.com/avalanria/go-avalanria/p2p/enr"
	"github.com/avalanria/go-avalanria/params"
	"github.com/avalanria/go-avalanria/rlp"
	"github.com/avalanria/go-avalanria/rpc"
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
	netRPCService  *avnapi.PublicNetAPI

	p2pServer  *p2p.Server
	p2pConfig  *p2p.Config
	udpEnabled bool
}

// New creates an instance of the light client.
func New(stack *node.Node, config *avnconfig.Config) (*LightAvalanria, error) {
	chainDb, err := stack.OpenDatabase("lightchaindata", config.DatabaseCache, config.DatabaseHandles, "avn/db/chaindata/", false)
	if err != nil {
		return nil, err
	}
	lesDb, err := stack.OpenDatabase("les.client", 0, 0, "avn/db/lesclient/", false)
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlockWithOverride(chainDb, config.Genesis, config.OverrideLondon)
	if _, isCompat := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !isCompat {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	peers := newServerPeerSet()
	lavn := &LightAvalanria{
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
		engine:         avnconfig.CreateConsensusEngine(stack, chainConfig, &config.Ethash, nil, false, chainDb),
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   core.NewBloomIndexer(chainDb, params.BloomBitsBlocksClient, params.HelperTrieConfirmations),
		p2pServer:      stack.Server(),
		p2pConfig:      &stack.Config().P2P,
		udpEnabled:     stack.Config().P2P.DiscoveryV5,
	}

	var prenegQuery vfc.QueryFunc
	if lavn.udpEnabled {
		prenegQuery = lavn.prenegQuery
	}
	lavn.serverPool, lavn.serverPoolIterator = vfc.NewServerPool(lesDb, []byte("serverpool:"), time.Second, prenegQuery, &mclock.System{}, config.UltraLightServers, requestList)
	lavn.serverPool.AddMetrics(suggestedTimeoutGauge, totalValueGauge, serverSelectableGauge, serverConnectedGauge, sessionValueMeter, serverDialedMeter)

	lavn.retriever = newRetrieveManager(peers, lavn.reqDist, lavn.serverPool.GetTimeout)
	lavn.relay = newLesTxRelay(peers, lavn.retriever)

	lavn.odr = NewLesOdr(chainDb, light.DefaultClientIndexerConfig, lavn.peers, lavn.retriever)
	lavn.chtIndexer = light.NewChtIndexer(chainDb, lavn.odr, params.CHTFrequency, params.HelperTrieConfirmations, config.LightNoPrune)
	lavn.bloomTrieIndexer = light.NewBloomTrieIndexer(chainDb, lavn.odr, params.BloomBitsBlocksClient, params.BloomTrieFrequency, config.LightNoPrune)
	lavn.odr.SetIndexers(lavn.chtIndexer, lavn.bloomTrieIndexer, lavn.bloomIndexer)

	checkpoint := config.Checkpoint
	if checkpoint == nil {
		checkpoint = params.TrustedCheckpoints[genesisHash]
	}
	// Note: NewLightChain adds the trusted checkpoint so it needs an ODR with
	// indexers already set but not started yet
	if lavn.blockchain, err = light.NewLightChain(lavn.odr, lavn.chainConfig, lavn.engine, checkpoint); err != nil {
		return nil, err
	}
	lavn.chainReader = lavn.blockchain
	lavn.txPool = light.NewTxPool(lavn.chainConfig, lavn.blockchain, lavn.relay)

	// Set up checkpoint oracle.
	lavn.oracle = lavn.setupOracle(stack, genesisHash, config)

	// Note: AddChildIndexer starts the update process for the child
	lavn.bloomIndexer.AddChildIndexer(lavn.bloomTrieIndexer)
	lavn.chtIndexer.Start(lavn.blockchain)
	lavn.bloomIndexer.Start(lavn.blockchain)

	// Start a light chain pruner to delete useless historical data.
	lavn.pruner = newPruner(chainDb, lavn.chtIndexer, lavn.bloomTrieIndexer)

	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		lavn.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}

	lavn.ApiBackend = &LesApiBackend{stack.Config().ExtRPCEnabled(), stack.Config().AllowUnprotectedTxs, lavn, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.Miner.GasPrice
	}
	lavn.ApiBackend.gpo = gasprice.NewOracle(lavn.ApiBackend, gpoParams)

	lavn.handler = newClientHandler(config.UltraLightServers, config.UltraLightFraction, checkpoint, lavn)
	if lavn.handler.ulc != nil {
		log.Warn("Ultra light client is enabled", "trustedNodes", len(lavn.handler.ulc.keys), "minTrustedFraction", lavn.handler.ulc.fraction)
		lavn.blockchain.DisableCheckFreq()
	}

	lavn.netRPCService = avnapi.NewPublicNetAPI(lavn.p2pServer, lavn.config.NetworkId)

	// Register the backend on the node
	stack.RegisterAPIs(lavn.APIs())
	stack.RegisterProtocols(lavn.Protocols())
	stack.RegisterLifecycle(lavn)

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
	return lavn, nil
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

// prenegQuery sends a capacity query to the given server node to determine whavner
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

// APIs returns the collection of RPC services the avalanria package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *LightAvalanria) APIs() []rpc.API {
	apis := avnapi.GetAPIs(s.ApiBackend)
	apis = append(apis, s.engine.APIs(s.BlockChain().HeaderChain())...)
	return append(apis, []rpc.API{
		{
			Namespace: "avn",
			Version:   "1.0",
			Service:   &LightDummyAPI{},
			Public:    true,
		}, {
			Namespace: "avn",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.handler.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "avn",
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
// light avalanria protocol implementation.
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
	log.Info("Light avalanria stopped")
	return nil
}

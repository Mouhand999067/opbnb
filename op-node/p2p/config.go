package p2p

import (
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/p2p/gating"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	ds "github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
	cmgr "github.com/libp2p/go-libp2p/p2p/net/connmgr"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
)

// TODO add values
var DefaultBootnodes = []*enode.Node{}

var OpBNBTestnet = big.NewInt(5611)

// OpBNBMainnetBootnodes are the enode URLs of the P2P bootstrap nodes running on
// the opBNB main network.
var OpBNBMainnetBootnodes = []string{
	// op-node
	"enr:-J24QIGeBCpZHdaQ5i8fyyK_uzaL1DdgOOZA3_lvjHvirjpgexMGkVeGMxlqauzEX7pWAfztCa9hpEGd_bS2a-1IqB6GAYvRDk5QgmlkgnY0gmlwhDaykUmHb3BzdGFja4PMAQCJc2VjcDI1NmsxoQJ-_5GZKjs7jaB4TILdgC8EwnwyL3Qip89wmjnyjvDDwoN0Y3CCIyuDdWRwgiMs",
	"enr:-J24QO_AhXy6stHsVvFWnECRD_1hMccZM6JqJgiXNVIRED1JRAJvIS48Pihku2z30TfizSGUAmeb22RQfPjW99hDu9WGAYvRBDnegmlkgnY0gmlwhDbjSM6Hb3BzdGFja4PMAQCJc2VjcDI1NmsxoQKetGQX7sXd4u8hZr6uayTZgHRDvGm36YaryqZkgnidS4N0Y3CCIyuDdWRwgiMs",
}

// OpBNBTestnetBootnodes are the enode URLs of the P2P bootstrap nodes running on
// the opBNB testnet network.
var OpBNBTestnetBootnodes = []string{
	// op-node
	"enr:-J24QGQBeMsXOaCCaLWtNFSfb2Gv50DjGOKToH2HUTAIn9yXImowlRoMDNuPNhSBZNQGCCE8eAl5O3dsONuuQp5Qix2GAYjB7KHSgmlkgnY0gmlwhDREiqaHb3BzdGFja4PrKwCJc2VjcDI1NmsxoQL4I9wpEVDcUb8bLWu6V8iPoN5w8E8q-GrS5WUCygYUQ4N0Y3CCIyuDdWRwgiMr",
	"enr:-J24QJKXHEkIhy0tmIk2EscMZ2aRrivNsZf_YhgIU51g4ZKHWY0BxW6VedRJ1jxmneW9v7JjldPOPpLkaNSo6cXGFxqGAYpK96oCgmlkgnY0gmlwhANzx96Hb3BzdGFja4PrKwCJc2VjcDI1NmsxoQMOCzUFffz04eyDrmkbaSCrMEvLvn5O4RZaZ5k1GV4wa4N0Y3CCIyuDdWRwgiMr",
}

type HostMetrics interface {
	gating.UnbanMetrics
	gating.ConnectionGaterMetrics
}

// SetupP2P provides a host and discovery service for usage in the rollup node.
type SetupP2P interface {
	Check() error
	Disabled() bool
	// Host creates a libp2p host service. Returns nil, nil if p2p is disabled.
	Host(log log.Logger, reporter metrics.Reporter, metrics HostMetrics) (host.Host, error)
	// Discovery creates a disc-v5 service. Returns nil, nil, nil if discovery is disabled.
	Discovery(log log.Logger, rollupCfg *rollup.Config, tcpPort uint16) (*enode.LocalNode, *discover.UDPv5, error)
	TargetPeers() uint
	BanPeers() bool
	BanThreshold() float64
	BanDuration() time.Duration
	GossipSetupConfigurables
	ReqRespSyncEnabled() bool
}

// ScoringParams defines the various types of peer scoring parameters.
type ScoringParams struct {
	PeerScoring        pubsub.PeerScoreParams
	ApplicationScoring ApplicationScoreParams
}

// Config sets up a p2p host and discv5 service from configuration.
// This implements SetupP2P.
type Config struct {
	Priv *crypto.Secp256k1PrivateKey

	DisableP2P  bool
	NoDiscovery bool

	ScoringParams *ScoringParams

	// Whether to ban peers based on their [PeerScoring] score. Should be negative.
	BanningEnabled bool
	// Minimum score before peers are disconnected and banned
	BanningThreshold float64
	BanningDuration  time.Duration

	ListenIP      net.IP
	ListenTCPPort uint16

	// Port to bind discv5 to
	ListenUDPPort uint16

	AdvertiseIP      net.IP
	AdvertiseTCPPort uint16
	AdvertiseUDPPort uint16
	Bootnodes        []*enode.Node
	DiscoveryDB      *enode.DB
	NetRestrict      *netutil.Netlist

	StaticPeers []core.Multiaddr

	HostMux             []libp2p.Option
	HostSecurity        []libp2p.Option
	NoTransportSecurity bool

	PeersLo    uint
	PeersHi    uint
	PeersGrace time.Duration

	MeshD     int // topic stable mesh target count
	MeshDLo   int // topic stable mesh low watermark
	MeshDHi   int // topic stable mesh high watermark
	MeshDLazy int // gossip target

	// FloodPublish publishes messages from ourselves to peers outside of the gossip topic mesh but supporting the same topic.
	FloodPublish bool

	// If true a NAT manager will host a NAT port mapping that is updated with PMP and UPNP by libp2p/go-nat
	NAT bool

	UserAgent string

	TimeoutNegotiation time.Duration
	TimeoutAccept      time.Duration
	TimeoutDial        time.Duration

	// Underlying store that hosts connection-gater and peerstore data.
	Store ds.Batching

	EnableReqRespSync bool
}

func DefaultConnManager(conf *Config) (connmgr.ConnManager, error) {
	return cmgr.NewConnManager(
		int(conf.PeersLo),
		int(conf.PeersHi),
		cmgr.WithGracePeriod(conf.PeersGrace),
		cmgr.WithSilencePeriod(time.Minute),
		cmgr.WithEmergencyTrim(true))
}

func (conf *Config) TargetPeers() uint {
	return conf.PeersLo
}

func (conf *Config) Disabled() bool {
	return conf.DisableP2P
}

func (conf *Config) PeerScoringParams() *ScoringParams {
	if conf.ScoringParams == nil {
		return nil
	}
	return conf.ScoringParams
}

func (conf *Config) BanPeers() bool {
	return conf.BanningEnabled
}

func (conf *Config) BanThreshold() float64 {
	return conf.BanningThreshold
}

func (conf *Config) BanDuration() time.Duration {
	return conf.BanningDuration
}

func (conf *Config) ReqRespSyncEnabled() bool {
	return conf.EnableReqRespSync
}

const maxMeshParam = 1000

func (conf *Config) Check() error {
	if conf.DisableP2P {
		return nil
	}
	if conf.Store == nil {
		return errors.New("p2p requires a persistent or in-memory peerstore, but found none")
	}
	if !conf.NoDiscovery {
		if conf.DiscoveryDB == nil {
			return errors.New("discovery requires a persistent or in-memory discv5 db, but found none")
		}
	}
	if conf.PeersLo == 0 || conf.PeersHi == 0 || conf.PeersLo > conf.PeersHi {
		return fmt.Errorf("peers lo/hi tides are invalid: %d, %d", conf.PeersLo, conf.PeersHi)
	}
	if conf.MeshD <= 0 || conf.MeshD > maxMeshParam {
		return fmt.Errorf("mesh D param must not be 0 or exceed %d, but got %d", maxMeshParam, conf.MeshD)
	}
	if conf.MeshDLo <= 0 || conf.MeshDLo > maxMeshParam {
		return fmt.Errorf("mesh Dlo param must not be 0 or exceed %d, but got %d", maxMeshParam, conf.MeshDLo)
	}
	if conf.MeshDHi <= 0 || conf.MeshDHi > maxMeshParam {
		return fmt.Errorf("mesh Dhi param must not be 0 or exceed %d, but got %d", maxMeshParam, conf.MeshDHi)
	}
	if conf.MeshDLazy <= 0 || conf.MeshDLazy > maxMeshParam {
		return fmt.Errorf("mesh Dlazy param must not be 0 or exceed %d, but got %d", maxMeshParam, conf.MeshDLazy)
	}
	return nil
}

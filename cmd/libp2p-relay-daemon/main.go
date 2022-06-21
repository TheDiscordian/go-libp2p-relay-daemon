package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"time"
	"crypto/tls"

	"github.com/libp2p/go-libp2p"
	relaydaemon "github.com/libp2p/go-libp2p-relay-daemon"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	relayv1 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv1/relay"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	connmgr "github.com/libp2p/go-libp2p/p2p/net/connmgr"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	ws "github.com/libp2p/go-libp2p/p2p/transport/websocket"

	_ "net/http/pprof"
)

func main() {
	idPath := flag.String("id", "identity", "identity key file path")
	cfgPath := flag.String("config", "", "json configuration file; empty uses the default configuration")
	flag.Parse()

	cfg, err := relaydaemon.LoadConfig(*cfgPath)
	if err != nil {
		panic(err)
	}
	privk, err := relaydaemon.LoadIdentity(*idPath)
	if err != nil {
		panic(err)
	}

	var opts []libp2p.Option

	opts = append(opts,
		libp2p.UserAgent("relayd/1.0"),
		libp2p.Identity(privk),
		libp2p.DisableRelay(),
		libp2p.ListenAddrStrings(cfg.Network.ListenAddrs...),
	)

	if len(cfg.Network.AnnounceAddrs) > 0 {
		var announce []ma.Multiaddr
		for _, s := range cfg.Network.AnnounceAddrs {
			a := ma.StringCast(s)
			announce = append(announce, a)
		}
		opts = append(opts,
			libp2p.AddrsFactory(func([]ma.Multiaddr) []ma.Multiaddr {
				return announce
			}),
		)
	} else {
		opts = append(opts,
			libp2p.AddrsFactory(func(addrs []ma.Multiaddr) []ma.Multiaddr {
				announce := make([]ma.Multiaddr, 0, len(addrs))
				for _, a := range addrs {
					if manet.IsPublicAddr(a) {
						announce = append(announce, a)
					}
				}
				return announce
			}),
		)
	}

	cm, err := connmgr.NewConnManager(
		cfg.ConnMgr.ConnMgrLo,
		cfg.ConnMgr.ConnMgrHi,
		connmgr.WithGracePeriod(cfg.ConnMgr.ConnMgrGrace),
	)
	if err != nil {
		panic(err)
	}

	/* TLS */

	certs := make([]tls.Certificate, len(cfg.TLS.KeyPairPaths))
	for i, keyPairPaths := range(cfg.TLS.KeyPairPaths) {
		certs[i], err = tls.LoadX509KeyPair(keyPairPaths[0], keyPairPaths[1])
		if err != nil {
			panic(err)
		}
	}

	opts = append(opts,
		libp2p.Transport(ws.New, ws.WithTLSConfig(&tls.Config{Certificates: certs})),
	)

	/* End of TLS */

	opts = append(opts,
		libp2p.ConnectionManager(cm),
	)

	host, err := libp2p.New(opts...)
	if err != nil {
		panic(err)
	}

	fmt.Printf("I am %s\n", host.ID())
	fmt.Println("Public Addresses:")
	for _, addr := range host.Addrs() {
		fmt.Printf("\t%s/p2p/%s\n", addr, host.ID())
	}

	go listenPprof(cfg.Daemon.PprofPort)
	time.Sleep(10 * time.Millisecond)

	acl, err := relaydaemon.NewACL(host, cfg.ACL)
	if err != nil {
		panic(err)
	}

	if cfg.RelayV1.Enabled {
		fmt.Println("Starting RelayV1...")

		_, err = relayv1.NewRelay(host,
			relayv1.WithResources(cfg.RelayV1.Resources),
			relayv1.WithACL(acl))
		if err != nil {
			panic(err)
		}
		fmt.Println("RelayV1 is running!")
	}

	if cfg.RelayV2.Enabled {
		fmt.Println("Starting RelayV2...")
		_, err = relayv2.New(host,
			relayv2.WithResources(cfg.RelayV2.Resources),
			relayv2.WithACL(acl))
		if err != nil {
			panic(err)
		}
		fmt.Printf("RelayV2 is running!\n")
	}

	/* PubSub */

	gs, err := pubsub.NewGossipSub(context.TODO(), host)
	if err != nil {
		panic(err)
	}
	announceCircuit, err := gs.Join("announce-circuit")
	if err != nil {
		panic(err)
	}
	sub, err := announceCircuit.Subscribe()
	if err != nil {
		panic(err)
	}
	go func() {
		var err error
		var msg *pubsub.Message
		for err == nil {
			msg, err = sub.Next(context.TODO())
			fmt.Println("[DEBUG]", string(msg.GetData()))
		}
		panic(err)
	}()

	go func() {
		for true {
			ps := host.Peerstore()
			//peerAddrs := ps.PeersWithAddrs()
			peerAddrs := ps.Peers()
			for _, peer := range peerAddrs {
				if peer == host.ID() {
					continue
				}
				fmt.Println("[DEBUG]", peer)
				addrs := ps.Addrs(peer)
				for _, addr := range addrs {
					protocols := addr.Protocols()
					for _, protocol := range protocols {
						if protocol.Code == 477 { // ws
							fmt.Println(peer)
						}
					}
				}
			}
			time.Sleep(time.Second)
		}
	}()

	/* End of PubSub */

	select {}
}

func listenPprof(p int) {
	if p == -1 {
		fmt.Printf("The pprof debug is disabled\n")
		return
	}
	addr := fmt.Sprintf("localhost:%d", p)
	fmt.Printf("Registering pprof debug http handler at: http://%s/debug/pprof/\n", addr)
	switch err := http.ListenAndServe(addr, nil); err {
	case nil:
		// all good, server is running and exited normally.
	case http.ErrServerClosed:
		// all good, server was shut down.
	default:
		// error, try another port
		fmt.Printf("error registering pprof debug http handler at: %s: %s\n", addr, err)
		panic(err)
	}
}

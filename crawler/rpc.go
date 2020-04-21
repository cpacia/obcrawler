package crawler

import (
	"context"
	"errors"
	"fmt"
	"github.com/cpacia/obcrawler/repo"
	"github.com/cpacia/obcrawler/rpc"
	"github.com/cpacia/obcrawler/rpc/pb"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"net"
	"net/http"
	"runtime"
	"strings"
)

// AuthenticationTokenKey is the key used in the context to authenticate clients.
// If this is set to anything other than "" in the config, then the server expects
// the client to set a key value in the context metadata to 'AuthenticationToken: cfg.AuthToken'
const AuthenticationTokenKey = "AuthenticationToken"

var authToken string

func newGrpcServer(netAddrs []net.Addr, crawler *Crawler, cfg *repo.Config) (*rpc.GrpcServer, error) {
	authToken = cfg.GrpcAuthToken
	for _, addr := range netAddrs {
		opts := []grpc.ServerOption{grpc.StreamInterceptor(interceptStreaming), grpc.UnaryInterceptor(interceptUnary)}
		creds, err := credentials.NewServerTLSFromFile(cfg.RPCCert, cfg.RPCKey)
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.Creds(creds))
		server := grpc.NewServer(opts...)

		allowAllOrigins := grpcweb.WithOriginFunc(func(origin string) bool {
			return true
		})
		wrappedGrpc := grpcweb.WrapServer(server, allowAllOrigins)

		handler := func(resp http.ResponseWriter, req *http.Request) {
			if wrappedGrpc.IsGrpcWebRequest(req) || wrappedGrpc.IsAcceptableGrpcCorsRequest(req) {
				wrappedGrpc.ServeHTTP(resp, req)
			} else {
				server.ServeHTTP(resp, req)
			}
		}

		httpServer := &http.Server{
			Addr:    addr.String(),
			Handler: http.HandlerFunc(handler),
		}

		gRPCServer := rpc.NewGrpcServer(crawler)
		pb.RegisterObcrawlerServer(server, gRPCServer)

		log.Infof("gRPC server listening on %s", addr)

		go func() {
			if err := httpServer.ListenAndServeTLS(cfg.RPCCert, cfg.RPCKey); err != nil {
				log.Debugf("Finished serving experimental gRPC: %v", err)
			}
		}()

		return gRPCServer, nil
	}
	return nil, nil
}

func interceptStreaming(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	p, ok := peer.FromContext(ss.Context())
	if ok {
		log.Infof("Streaming method %s invoked by %s", info.FullMethod,
			p.Addr.String())
	}

	err := validateAuthenticationToken(ss.Context())
	if err != nil {
		return err
	}

	err = handler(srv, ss)
	if err != nil && ok {
		log.Errorf("Streaming method %s invoked by %s errored: %v",
			info.FullMethod, p.Addr.String(), err)
	}
	return err
}

func interceptUnary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	p, ok := peer.FromContext(ctx)
	if ok {
		log.Infof("Unary method %s invoked by %s", info.FullMethod,
			p.Addr.String())
	}

	err = validateAuthenticationToken(ctx)
	if err != nil {
		return nil, err
	}

	resp, err = handler(ctx, req)
	if err != nil && ok {
		log.Errorf("Unary method %s invoked by %s errored: %v",
			info.FullMethod, p.Addr.String(), err)
	}
	return resp, err
}

func validateAuthenticationToken(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if authToken != "" && (!ok || len(md.Get(AuthenticationTokenKey)) == 0 || md.Get(AuthenticationTokenKey)[0] != authToken) {
		return errors.New("invalid authentication token")
	}
	return nil
}

// parseListeners determines whether each listen address is IPv4 and IPv6 and
// returns a slice of appropriate net.Addrs to listen on with TCP. It also
// properly detects addresses which apply to "all interfaces" and adds the
// address as both IPv4 and IPv6.
func parseListeners(addrs []string) ([]net.Addr, error) {
	netAddrs := make([]net.Addr, 0, len(addrs)*2)
	for _, addr := range addrs {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			// Shouldn't happen due to already being normalized.
			return nil, err
		}

		// Empty host or host of * on plan9 is both IPv4 and IPv6.
		if host == "" || (host == "*" && runtime.GOOS == "plan9") {
			netAddrs = append(netAddrs, simpleAddr{net: "tcp4", addr: addr})
			netAddrs = append(netAddrs, simpleAddr{net: "tcp6", addr: addr})
			continue
		}

		// Strip IPv6 zone id if present since net.ParseIP does not
		// handle it.
		zoneIndex := strings.LastIndex(host, "%")
		if zoneIndex > 0 {
			host = host[:zoneIndex]
		}

		// Parse the IP.
		ip := net.ParseIP(host)
		if ip == nil {
			return nil, fmt.Errorf("'%s' is not a valid IP address", host)
		}

		// To4 returns nil when the IP is not an IPv4 address, so use
		// this determine the address type.
		if ip.To4() == nil {
			netAddrs = append(netAddrs, simpleAddr{net: "tcp6", addr: addr})
		} else {
			netAddrs = append(netAddrs, simpleAddr{net: "tcp4", addr: addr})
		}
	}
	return netAddrs, nil
}

// simpleAddr implements the net.Addr interface with two struct fields
type simpleAddr struct {
	net, addr string
}

// String returns the address.
//
// This is part of the net.Addr interface.
func (a simpleAddr) String() string {
	return a.addr
}

// Network returns the network.
//
// This is part of the net.Addr interface.
func (a simpleAddr) Network() string {
	return a.net
}

// Ensure simpleAddr implements the net.Addr interface.
var _ net.Addr = simpleAddr{}

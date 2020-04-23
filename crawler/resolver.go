package crawler

import (
	"bytes"
	"fmt"
	"github.com/cpacia/obcrawler/repo"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	peer "github.com/libp2p/go-libp2p-peer"
	"net"
	"net/http"
)

type resolver struct {
	db *repo.Database
}

func newResolver(netAddrs []net.Addr, db *repo.Database, cfg *repo.Config) *resolver {
	res := &resolver{db: db}
	for _, addr := range netAddrs {

		r := mux.NewRouter()
		r.HandleFunc("/ipns/{peerID}", res.handler).Methods("GET")
		r.Use(mux.CORSMethodMiddleware(r))

		httpServer := &http.Server{
			Addr:    addr.String(),
			Handler: r,
		}

		log.Infof("Resolver server listening on %s", addr)

		go func() {
			if cfg.NoResolverTLS {
				if err := httpServer.ListenAndServe(); err != nil {
					log.Debugf("Finished serving resolver: %v", err)
				}
			} else {
				if err := httpServer.ListenAndServeTLS(cfg.RPCCert, cfg.RPCKey); err != nil {
					log.Debugf("Finished serving resolver: %v", err)
				}
			}
		}()
		return res
	}
	return nil
}

func (res *resolver) handler(w http.ResponseWriter, r *http.Request) {
	peerIDStr := mux.Vars(r)["peerID"]

	pid, err := peer.IDB58Decode(peerIDStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var peerRecord repo.Peer
	err = res.db.View(func(db *gorm.DB) error {
		return db.Where("peer_id=?", pid.Pretty()).First(&peerRecord).Error
	})
	if err != nil && gorm.IsRecordNotFoundError(err) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/protobuf; proto=ipns.pb.IpnsEntry")
	fmt.Fprint(w, bytes.NewReader(peerRecord.IPNSRecord))
}

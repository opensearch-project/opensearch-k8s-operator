package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"sync/atomic"

	"k8s.io/client-go/tools/leaderelection"
	rl "k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	// Name of the lock to use (depends on if it is per-nodepool or for the entire Opensearch cluster)
	lockName      = os.Getenv("LOCK_NAME")
	lockNamespace = os.Getenv("CLUSTER_NAMESPACE")

	// Port Opensearch HTTP API is listening on, for readiness checks
	opensearchHttpPort = os.Getenv("HTTP_PORT")

	// unique identity for the leader election
	identity = os.Getenv("POD_NAME")
)

func main() {
	// Get the active kubernetes context, uses in-cluster by default
	cfg, err := ctrl.GetConfig()
	if err != nil {
		// If we can't get a context, bail out as there is no way to recover
		panic(err.Error())
	}

	// Create a new lock. This will be used to create a Lease resource in the cluster.
	l, err := rl.NewFromKubeconfig(
		rl.LeasesResourceLock,
		lockNamespace,
		lockName,
		rl.ResourceLockConfig{
			Identity: identity,
		},
		cfg,
		time.Second*5,
	)
	if err != nil {
		// If we can't create the Lock, bail out as there is no way to recover
		panic(err)
	}

	// Track if we are the leader
	leader := &atomic.Bool{}

	// https://pkg.go.dev/k8s.io/client-go/tools/leaderelection#LeaderElectionConfig
	el, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          l,
		LeaseDuration: time.Second * 10,
		RenewDeadline: time.Second * 5,
		RetryPeriod:   time.Second * 2,
		Name:          lockName,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				// We are now leader
				leader.Store(true)
			},
			OnStoppedLeading: func() {
				// We are no longer leader
				leader.Store(false)
			},
			OnNewLeader: func(identity string) {},
		},
	})
	if err != nil {
		// If we can't create the LeaderElection, bail out as there is no way to recover
		panic(err)
	}

	// start http server in the background
	go healthserver(leader)

	// Begin the leader election process. This will block.
	el.Run(context.Background())
}

func healthserver(leader *atomic.Bool) {
	// self health
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprintf(w, "OK"); err != nil {
			fmt.Printf("Failed to write to response: %s", err)
		}
	})
	// readiness of Opensearch cluster
	http.HandleFunc("/cluster_readiness", func(w http.ResponseWriter, r *http.Request) {
		isLeader := leader.Load()
		status, err := callClusterHealthEndpoint(isLeader)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := fmt.Fprintf(w, "Failed to check cluster health: %s: %s", status, err); err != nil {
				fmt.Printf("Failed to write to response: %s", err)
			}
			return
		}
		if isLeader {
			if status != "green" {
				w.WriteHeader(http.StatusServiceUnavailable)
			} else {
				w.WriteHeader(http.StatusOK)
			}
			if _, err := fmt.Fprintf(w, "Cluster status is %s", status); err != nil {
				fmt.Printf("Failed to write to response: %s", err)
			}
		} else {
			// if we are not the leader we are only concerned with if opensearch is reachable
			w.WriteHeader(http.StatusOK)
			if _, err := fmt.Fprintf(w, "Cluster status is %s", status); err != nil {
				fmt.Printf("Failed to write to response: %s", err)
			}
		}
	})

	s := &http.Server{
		Addr:         ":8123",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Fatal(s.ListenAndServe())
}

func dialTimeout(network, addr string) (net.Conn, error) {
	// Fail fast
	return net.DialTimeout(network, addr, time.Duration(1*time.Second))
}

func httpClient() http.Client {
	transport := http.Transport{
		Dial:            dialTimeout,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		// These options are needed as otherwise connections would be kept and leak memory
		DisableKeepAlives: true,
		MaxIdleConns:      1,
	}

	return http.Client{
		Transport: &transport,
		Timeout:   time.Duration(2 * time.Second),
	}
}

func callClusterHealthEndpoint(checkStatus bool) (string, error) {
	username, err := os.ReadFile("/mnt/admin-credentials/username")
	if err != nil {
		return "no_username", err
	}
	password, err := os.ReadFile("/mnt/admin-credentials/password")
	if err != nil {
		return "no_password", err
	}
	client := httpClient()
	resp, err := client.Get(fmt.Sprintf("https://%s:%s@localhost:%s/_cluster/health", username, password, opensearchHttpPort))
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Could not close response body: %s", err)
		}
	}()

	// we care about the actual status only if we are the leader, otherwise reachability is enough
	if checkStatus {
		if resp.StatusCode == 200 {
			var response ClusterHealthResponse
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				return "", err
			}
			return response.Status, nil
		} else {
			return "error", nil
		}
	} else {
		return "", nil
	}
}

// minimal response struct with only the field we need
// other fields get ignored
type ClusterHealthResponse struct {
	Status string `json:"status,omitempty"`
}

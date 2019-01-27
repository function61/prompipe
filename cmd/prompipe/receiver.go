package main

import (
	"context"
	"github.com/function61/gokit/envvar"
	"github.com/function61/gokit/ossignal"
	"github.com/function61/gokit/stopper"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"io/ioutil"
	"log"
	"net/http"
)

const (
	promContentType = "text/plain; version=0.0.4; charset=utf-8"
)

type ReceiverConfig struct {
	ExpectedBearerToken string
}

func readReceiverConfig() (*ReceiverConfig, error) {
	token, err := envvar.Get("BEARER_TOKEN")
	if err != nil {
		return nil, err
	}

	return &ReceiverConfig{
		ExpectedBearerToken: token,
	}, nil
}

func runReceiver(stop *stopper.Stopper) error {
	conf, err := readReceiverConfig()
	if err != nil {
		return err
	}

	// TODO: mutex
	state := map[string][]byte{}

	putHandler := func(w http.ResponseWriter, r *http.Request) {
		job := mux.Vars(r)["job"]
		instance := mux.Vars(r)["instance"]

		if r.Header.Get("Authorization") != "Bearer "+conf.ExpectedBearerToken {
			http.Error(w, "incorrect or missing bearer token", http.StatusForbidden)
			return
		}

		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}

		key := jobAndInstanceKey(job, instance)

		state[key] = buf
	}

	getHandler := func(w http.ResponseWriter, r *http.Request) {
		job := mux.Vars(r)["job"]
		instance := mux.Vars(r)["instance"]

		key := jobAndInstanceKey(job, instance)

		body, found := state[key]
		if !found {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", promContentType)

		if _, err := w.Write(body); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	authenticatedRouter := mux.NewRouter()
	authenticatedRouter.Methods(http.MethodPut).Path("/metrics/{job}/{instance}").HandlerFunc(putHandler)

	unauthenticatedRouter := mux.NewRouter()
	unauthenticatedRouter.Methods(http.MethodGet).Path("/metrics/{job}/{instance}").HandlerFunc(getHandler)

	// consult README for rationale of this two-port system
	authenticatedServer := http.Server{
		Addr:    ":80",
		Handler: authenticatedRouter,
	}

	unauthenticatedServer := http.Server{
		Addr:    ":9090",
		Handler: unauthenticatedRouter,
	}

	go func() {
		defer stop.Done()
		<-stop.Signal

		if err := authenticatedServer.Shutdown(context.TODO()); err != nil {
			panic(err)
		}

		if err := unauthenticatedServer.Shutdown(context.TODO()); err != nil {
			panic(err)
		}
	}()

	listenAndServeResult := make(chan error, 2)

	runOneServer := func(srv *http.Server) error {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}
		return nil
	}

	runOneServerWithResultChan := func(srv *http.Server) {
		listenAndServeResult <- runOneServer(srv)
	}

	go runOneServerWithResultChan(&authenticatedServer)
	go runOneServerWithResultChan(&unauthenticatedServer)

	// wait for both servers to exit, and check their errors
	for i := 0; i < 2; i++ {
		if err := <-listenAndServeResult; err != nil {
			return err
		}
	}

	return nil
}

func receiverEntry() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "receiver",
		Short: "Starts the receiver",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			workers := stopper.NewManager()

			go func() {
				log.Printf("Received signal %s; stopping", <-ossignal.InterruptOrTerminate())
				// logl.Info.Printf("Received signal %s; stopping", <-ossignal.InterruptOrTerminate())

				workers.StopAllWorkersAndWait()
			}()

			if err := runReceiver(workers.Stopper()); err != nil {
				panic(err)
			}
		},
	}

	return cmd
}

func jobAndInstanceKey(job string, instance string) string {
	return job + ":" + instance
}

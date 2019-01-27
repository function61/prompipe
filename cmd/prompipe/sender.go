package main

import (
	"context"
	"fmt"
	"github.com/function61/gokit/cryptorandombytes"
	"github.com/function61/gokit/ezhttp"
	"github.com/function61/gokit/jsonfile"
	"github.com/function61/gokit/ossignal"
	"github.com/function61/gokit/stopper"
	"github.com/function61/gokit/systemdinstaller"
	"github.com/spf13/cobra"
	"log"
	"os"
	"time"
)

type Pair struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type Config struct {
	BearerToken string `json:"bearer_token"`
	Pairs       []Pair `json:"pairs"`
}

// GETs metrics from source
// PUTs metrics to destination (authenticates with a bearer token)
func pipePrometheusMetrics(pair Pair, conf Config) error {
	// this timeout is combined for both "source" and "destination"
	ctx, cancel := context.WithTimeout(context.TODO(), ezhttp.DefaultTimeout10s)
	defer cancel()

	fromResponse, err := ezhttp.Get(
		ctx,
		pair.Source)
	if err != nil {
		return fmt.Errorf("GET failed for %s: %v", pair.Source, err)
	}

	// pipe it to destination
	_, err = ezhttp.Put(
		ctx,
		pair.Destination,
		ezhttp.AuthBearer(conf.BearerToken),
		ezhttp.SendBody(fromResponse.Body, promContentType))
	if err != nil {
		return fmt.Errorf("PUT failed for %s: %v", pair.Destination, err)
	}

	return nil
}

func pipePrometheusMetricsWithChan(pair Pair, conf Config, result chan<- error) {
	result <- pipePrometheusMetrics(pair, conf)
}

func runSender(conf Config, stop *stopper.Stopper) error {
	defer stop.Done()

	fourSeconds := time.NewTicker(4 * time.Second)

	results := make(chan error, len(conf.Pairs))

	for {
		select {
		case <-stop.Signal:
			return nil
		case <-fourSeconds.C:
			for _, pair := range conf.Pairs {
				// run in parallel
				go pipePrometheusMetricsWithChan(pair, conf, results)
			}

			// collect results
			for i := 0; i < len(conf.Pairs); i++ {
				if err := <-results; err != nil {
					log.Println(err.Error())
				}
			}
		}
	}
}

func senderEntry() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sender",
		Short: "Starts the sender",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			workers := stopper.NewManager()

			conf := Config{}
			if err := jsonfile.Read("config.json", &conf, true); err != nil {
				panic(err)
			}

			go func() {
				log.Printf("Received signal %s; stopping", <-ossignal.InterruptOrTerminate())
				// logl.Info.Printf("Received signal %s; stopping", <-ossignal.InterruptOrTerminate())

				workers.StopAllWorkersAndWait()
			}()

			if err := runSender(conf, workers.Stopper()); err != nil {
				panic(err)
			}
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Installs systemd unit file to make this start on system boot",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			hints, err := systemdinstaller.InstallSystemdServiceFile(
				"prompipe-sender",
				[]string{"sender"},
				"Prometheus pipe sender")

			if err != nil {
				panic(err)
			} else {
				fmt.Println(hints)
			}
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "exampleconfig",
		Short: "Prints an example config file",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			exampleConfig := Config{
				BearerToken: cryptorandombytes.Base64Url(16),
				Pairs: []Pair{
					{
						Source:      "http://192.168.1.100:9090/metrics",
						Destination: "http://promremotereceiver.example.com/metrics/fooproject/192.168.1.100",
					},
				},
			}

			if err := jsonfile.Marshal(os.Stdout, &exampleConfig); err != nil {
				panic(err)
			}
		},
	})

	return cmd
}

package cmd

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/well"
	"github.com/cybozu/neco-containers/ingress-watcher/pkg/watch"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var pushConfig struct {
	JobName      string
	PushAddr     string
	PushInterval time.Duration
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push metrics to Pushgateway",
	Long:  `Push metrics to Pushgateway`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if configFile != "" {
			if err := viper.Unmarshal(&pushConfig); err != nil {
				return err
			}
		}

		if len(pushConfig.JobName) == 0 {
			return errors.New("required flag \"job-name\" not set")
		}

		if len(pushConfig.PushAddr) == 0 {
			return errors.New("required flag \"push-addr\" not set")
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		well.Go(watch.NewWatcher(
			rootConfig.TargetURLs,
			rootConfig.WatchInterval,
			&http.Client{},
		).Run)
		well.Go(func(ctx context.Context) error {
			tick := time.NewTicker(pushConfig.PushInterval)
			defer tick.Stop()

			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-tick.C:
					pusher := push.New(pushConfig.PushAddr, pushConfig.JobName).Gatherer(registry)
					err := pusher.Add()
					if err != nil {
						log.Warn("push failed.", map[string]interface{}{
							"addr":      pushConfig.PushAddr,
							log.FnError: err,
						})
					} else {
						log.Info("push succeeded.", map[string]interface{}{
							"addr": pushConfig.PushAddr,
						})
					}
				}
			}

		})
		well.Stop()
		err := well.Wait()
		if err != nil && !well.IsSignaled(err) {
			log.ErrorExit(err)
		}
	},
}

func init() {
	fs := pushCmd.Flags()
	fs.StringVarP(&pushConfig.PushAddr, "push-addr", "", "", "Pushgateway addres.")
	fs.StringVarP(&pushConfig.JobName, "job-name", "", "", "Job name.")
	fs.DurationVarP(&pushConfig.PushInterval, "push-interval", "", 10*time.Second, "Push interval.")

	rootCmd.AddCommand(pushCmd)
}

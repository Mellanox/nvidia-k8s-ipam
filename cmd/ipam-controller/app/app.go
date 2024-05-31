/*
 Copyright 2023, NVIDIA CORPORATION & AFFILIATES
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
     http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// Package app does all the work necessary to configure and run a
// IPAM Controller app process.
package app

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	// register json format for logger
	_ "k8s.io/component-base/logs/json/register"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
	"github.com/Mellanox/nvidia-k8s-ipam/cmd/ipam-controller/app/options"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/common"
	cidrpoolctrl "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/controllers/cidrpool"
	ippoolctrl "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/controllers/ippool"
	nodectrl "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/controllers/node"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/migrator"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/version"
)

// NewControllerCommand creates a *cobra.Command object with default parameters
func NewControllerCommand() *cobra.Command {
	opts := options.New()
	ctx := ctrl.SetupSignalHandler()

	cmd := &cobra.Command{
		Use:          common.IPAMName + " controller",
		Long:         `NVIDIA K8S IPAM Controller`,
		SilenceUsage: true,
		Version:      version.GetVersionString(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Validate(); err != nil {
				return fmt.Errorf("invalid config: %v", err)
			}
			conf, err := ctrl.GetConfig()
			if err != nil {
				return fmt.Errorf("failed to read config for k8s client: %v", err)
			}
			return RunController(logr.NewContext(ctx, klog.NewKlogr()), conf, opts)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}
	sharedFS := cliflag.NamedFlagSets{}
	opts.AddNamedFlagSets(&sharedFS)

	cmdFS := cmd.PersistentFlags()
	for _, f := range sharedFS.FlagSets {
		cmdFS.AddFlagSet(f)
	}

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cliflag.SetUsageAndHelpFunc(cmd, sharedFS, cols)

	return cmd
}

// RunController start IPAM controller with provided options
func RunController(ctx context.Context, config *rest.Config, opts *options.Options) error {
	logger := logr.FromContextOrDiscard(ctx)
	ctrl.SetLogger(logger)

	logger.Info("start IPAM controller",
		"version", version.GetVersionString(), "poolsNamespace", opts.IPPoolsNamespace)

	scheme := runtime.NewScheme()

	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		logger.Error(err, "failed to register scheme")
		return err
	}

	if err := ipamv1alpha1.AddToScheme(scheme); err != nil {
		logger.Error(err, "failed to register ipamv1alpha1 scheme")
		return err
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                        scheme,
		Metrics:                       metricsserver.Options{BindAddress: opts.MetricsAddr},
		HealthProbeBindAddress:        opts.ProbeAddr,
		LeaderElection:                opts.EnableLeaderElection,
		LeaderElectionNamespace:       opts.LeaderElectionNamespace,
		LeaderElectionID:              "dd1643cf.nvidia.com",
		LeaderElectionReleaseOnCancel: true,
		Cache:                         cache.Options{DefaultNamespaces: map[string]cache.Config{opts.IPPoolsNamespace: {}}},
	})

	if err != nil {
		logger.Error(err, "unable to start manager")
		return err
	}

	k8sClient, err := client.New(config,
		client.Options{Scheme: mgr.GetScheme(), Mapper: mgr.GetRESTMapper()})
	if err != nil {
		logger.Error(err, "failed to create k8sClient client")
		os.Exit(1)
	}

	migrationChan := make(chan struct{})
	m := migrator.Migrator{
		IPPoolsNamespace: opts.IPPoolsNamespace,
		K8sClient:        k8sClient,
		MigrationCh:      migrationChan,
		LeaderElection:   opts.EnableLeaderElection,
		Logger:           logger.WithName("Migrator"),
	}
	err = mgr.Add(&m)
	if err != nil {
		logger.Error(err, "failed to add Migrator to the Manager")
		os.Exit(1)
	}

	ipPoolNodeEventCH := make(chan event.GenericEvent, 1)
	cidrPoolNodeEventCH := make(chan event.GenericEvent, 1)

	if err = (&nodectrl.NodeReconciler{
		IPPoolNodeEventCH:   ipPoolNodeEventCH,
		CIDRPoolNodeEventCH: cidrPoolNodeEventCH,
		MigrationCh:         migrationChan,
		PoolsNamespace:      opts.IPPoolsNamespace,
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create controller", "controller", "Node")
		return err
	}

	if opts.EnableWebHook {
		if err = (&ipamv1alpha1.IPPool{}).SetupWebhookWithManager(mgr); err != nil {
			logger.Error(err, "unable to create webhook", "webhook", "IPPool")
			os.Exit(1)
		}
		if err = (&ipamv1alpha1.CIDRPool{}).SetupWebhookWithManager(mgr); err != nil {
			logger.Error(err, "unable to create webhook", "webhook", "CIDRPool")
			os.Exit(1)
		}
	}
	if err = (&ippoolctrl.IPPoolReconciler{
		NodeEventCh:    ipPoolNodeEventCH,
		PoolsNamespace: opts.IPPoolsNamespace,
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		MigrationCh:    migrationChan,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create controller", "controller", "IPPool")
		return err
	}

	if err = (&cidrpoolctrl.CIDRPoolReconciler{
		NodeEventCh:    cidrPoolNodeEventCH,
		PoolsNamespace: opts.IPPoolsNamespace,
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create controller", "controller", "CIDRPool")
		return err
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Error(err, "unable to set up health check")
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logger.Error(err, "unable to set up ready check")
		return err
	}

	if err := mgr.Start(ctx); err != nil {
		logger.Error(err, "problem running manager")
		return err
	}
	logger.Info("IPAM controller stopped")
	return nil
}

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
	"net"
	"path/filepath"
	"sync"

	"github.com/go-logr/logr"
	"github.com/google/renameio/v2"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	// register json format for logger
	_ "k8s.io/component-base/logs/json/register"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	daemonv1 "github.com/Mellanox/nvidia-k8s-ipam/api/grpc/nvidia/ipam/daemon/v1"
	"github.com/Mellanox/nvidia-k8s-ipam/cmd/ipam-daemon/app/options"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cmdutils"
	cniTypes "github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/common"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-daemon/allocator"
	nodectrl "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-daemon/controllers/node"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-daemon/grpc/middleware"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-daemon/handlers"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-daemon/store"
	poolPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/version"
)

// NewControllerCommand creates a *cobra.Command object with default parameters
func NewControllerCommand() *cobra.Command {
	opts := options.New()
	ctx := ctrl.SetupSignalHandler()

	cmd := &cobra.Command{
		Use:          common.IPAMName + " daemon",
		Long:         `NVIDIA K8S IPAM Daemon`,
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
			return RunDaemon(logr.NewContext(ctx, klog.NewKlogr()), conf, opts)
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

// RunDaemon start IPAM daemon with provided options
//
//nolint:funlen
func RunDaemon(ctx context.Context, config *rest.Config, opts *options.Options) error {
	logger := logr.FromContextOrDiscard(ctx)
	ctrl.SetLogger(logger)

	logger.Info("start IPAM daemon",
		"version", version.GetVersionString(), "node", opts.NodeName)

	if err := deployShimCNI(logger, opts); err != nil {
		return err
	}

	scheme := runtime.NewScheme()

	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		logger.Error(err, "failed to register scheme")
		return err
	}

	poolManager := poolPkg.NewManager()

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme: scheme,
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: cache.SelectorsByObject{&corev1.Node{}: cache.ObjectSelector{
				Field: fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", opts.NodeName)),
			}},
		}),
		MetricsBindAddress:     opts.MetricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: opts.ProbeAddr,
	})
	if err != nil {
		logger.Error(err, "unable to initialize manager")
		return err
	}
	if err = (&nodectrl.NodeReconciler{
		PoolManager: poolManager,
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create controller", "controller", "Node")
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

	grpcServer, listener, err := initGRPCServer(opts, logger, poolManager)
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	wg.Add(3)

	errCh := make(chan error, 1)

	innerCtx, innerCFunc := context.WithCancel(ctx)
	defer innerCFunc()

	go func() {
		defer wg.Done()
		<-innerCtx.Done()
		grpcServer.GracefulStop()
	}()
	go func() {
		defer wg.Done()
		logger.Info("start grpc server")
		if err := grpcServer.Serve(listener); err != nil {
			logger.Error(err, "problem start grpc server")
			select {
			case errCh <- err:
			default:
			}
		}
		logger.Info("grpc server stopped")
	}()
	go func() {
		defer wg.Done()
		logger.Info("start manager")
		if err := mgr.Start(innerCtx); err != nil {
			logger.Error(err, "problem running manager")
			select {
			case errCh <- err:
			default:
			}
		}
		logger.Info("manager stopped")
	}()

	select {
	case <-ctx.Done():
	case <-errCh:
		innerCFunc()
	}
	wg.Wait()

	logger.Info("IPAM daemon stopped")
	return nil
}

func initGRPCServer(opts *options.Options,
	log logr.Logger, poolManager poolPkg.Manager) (*grpc.Server, net.Listener, error) {
	network, address, err := options.ParseBindAddress(opts.BindAddress)
	if err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		log.Error(err, "failed to start listener for GRPC server")
		return nil, nil, err
	}
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		middleware.SetLoggerMiddleware,
		middleware.LogRequestMiddleware,
		middleware.LogResponseMiddleware))

	daemonv1.RegisterIPAMBackendServiceServer(grpcServer,
		handlers.New(poolManager, store.NewManager(opts.StoreFile), allocator.NewIPAllocator))
	return grpcServer, listener, nil
}

func deployShimCNI(log logr.Logger, opts *options.Options) error {
	// copy nv-ipam binary
	if !opts.CNISkipBinFileCopy {
		// Copy
		if err := cmdutils.CopyFileAtomic(opts.CNIBinFile, opts.CNIBinDir,
			"_nv-ipam", "nv-ipam"); err != nil {
			log.Error(err, "failed at nv-ipam copy")
			return err
		}
	}
	return createNVIPAMConfig(log, opts)
}

func createNVIPAMConfig(log logr.Logger, opts *options.Options) error {
	cfg := fmt.Sprintf(`{
  "daemonSocket":    "%s",
  "daemonCallTimeout":    %d,
  "logFile":   "%s",
  "logLevel": "%s"
}
`, opts.CNIDaemonSocket, opts.CNIDaemonCallTimeout, opts.CNILogFile, opts.CNILogLevel)

	err := renameio.WriteFile(filepath.Join(opts.CNIConfDir, cniTypes.ConfFileName), []byte(cfg), 0664)
	if err != nil {
		log.Error(err, "failed to write configuration for shim CNI")
		return err
	}
	log.Info("config for shim CNI written", "config", cfg)
	return nil
}

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
// IPAM Node daemon app process.
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/gofrs/flock"
	"github.com/google/renameio/v2"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	// register json format for logger
	_ "k8s.io/component-base/logs/json/register"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	nodev1 "github.com/Mellanox/nvidia-k8s-ipam/api/grpc/nvidia/ipam/node/v1"
	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
	"github.com/Mellanox/nvidia-k8s-ipam/cmd/ipam-node/app/options"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cmdutils"
	cniTypes "github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/common"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/allocator"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/cleaner"
	ippoolctrl "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/controllers/ippool"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/grpc/middleware"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/handlers"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/migrator"
	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
	poolPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/version"
)

// NewControllerCommand creates a *cobra.Command object with default parameters
func NewControllerCommand() *cobra.Command {
	opts := options.New()
	ctx := ctrl.SetupSignalHandler()

	cmd := &cobra.Command{
		Use:          common.IPAMName + "node daemon",
		Long:         `NVIDIA K8S IPAM Node Daemon`,
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
			return RunNodeDaemon(logr.NewContext(ctx, klog.NewKlogr()), conf, opts)
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

// RunNodeDaemon start IPAM node daemon with provided options
//
//nolint:funlen
func RunNodeDaemon(ctx context.Context, config *rest.Config, opts *options.Options) error {
	logger := logr.FromContextOrDiscard(ctx)
	ctrl.SetLogger(logger)

	logger.Info("start IPAM node daemon",
		"version", version.GetVersionString(), "node", opts.NodeName,
		"IPPools Namespace", opts.PoolsNamespace)

	if err := deployShimCNI(logger, opts); err != nil {
		return err
	}

	scheme := runtime.NewScheme()

	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		logger.Error(err, "failed to register scheme")
		return err
	}

	if err := ipamv1alpha1.AddToScheme(scheme); err != nil {
		logger.Error(err, "failed to register ipamv1alpha1 scheme")
		return err
	}

	poolManager := poolPkg.NewManager()

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: opts.MetricsAddr},
		HealthProbeBindAddress: opts.ProbeAddr,
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{opts.PoolsNamespace: {}},
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Pod{}: {Namespaces: map[string]cache.Config{cache.AllNamespaces: {}}}},
		},
	})
	if err != nil {
		logger.Error(err, "unable to initialize manager")
		return err
	}

	if err = (&ippoolctrl.IPPoolReconciler{
		PoolManager: poolManager,
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		NodeName:    opts.NodeName,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create controller", "controller", "IPPool")
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

	storeLock := flock.New(opts.StoreFile)
	locked, err := storeLock.TryLock()
	if err != nil {
		logger.Error(err, "failed to set store lock")
		return err
	}
	if !locked {
		err := fmt.Errorf("store lock is held by a different process")
		logger.Error(err, "failed to lock store")
		return err
	}
	defer func() {
		err := storeLock.Unlock()
		if err != nil {
			logger.Error(err, "failed to release store lock")
		}
	}()
	store := storePkg.New(opts.StoreFile)
	// do initial store loading, to validate stored data
	s, err := store.Open(ctx)
	if err != nil {
		logger.Error(err, "failed to validate store data")
		return err
	}
	s.Cancel()
	if err := migrator.New(store).Migrate(ctx); err != nil {
		logger.Error(err, fmt.Sprintf("failed to migrate host-local IPAM store, "+
			"set %s env variable to disable migration", migrator.EnvDisableMigration))
		return err
	}

	grpcServer, listener, err := initGRPCServer(opts, logger, poolManager, store)
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	errCh := make(chan error, 1)

	innerCtx, innerCFunc := context.WithCancel(ctx)
	defer innerCFunc()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-innerCtx.Done()
		grpcServer.GracefulStop()
	}()
	wg.Add(1)
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
	wg.Add(1)
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
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("start stale reservations cleaner")
		if !mgr.GetCache().WaitForCacheSync(innerCtx) {
			select {
			case errCh <- fmt.Errorf("failed to sync informer cache"):
			default:
			}
			return
		}
		c := cleaner.New(mgr.GetClient(), store, poolManager, time.Minute, 3)
		c.Start(innerCtx)
		logger.Info("cleaner stopped")
	}()

	select {
	case <-ctx.Done():
	case <-errCh:
		innerCFunc()
	}
	wg.Wait()

	logger.Info("IPAM node daemon stopped")
	return nil
}

func initGRPCServer(opts *options.Options,
	log logr.Logger, poolConfReader poolPkg.ConfigReader, store storePkg.Store) (*grpc.Server, net.Listener, error) {
	network, address, err := options.ParseBindAddress(opts.BindAddress)
	if err != nil {
		return nil, nil, err
	}
	if err := cleanUNIXSocketIfRequired(opts); err != nil {
		log.Error(err, "failed to clean socket path")
		return nil, nil, err
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		log.Error(err, "failed to start listener for GRPC server")
		return nil, nil, err
	}
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		middleware.SetLoggerMiddleware,
		middleware.LogCallMiddleware))

	nodev1.RegisterIPAMServiceServer(grpcServer,
		handlers.New(poolConfReader, store, allocator.NewIPAllocator))
	return grpcServer, listener, nil
}

func cleanUNIXSocketIfRequired(opts *options.Options) error {
	socketPrefix := "unix://"
	if !strings.HasPrefix(opts.BindAddress, socketPrefix) {
		return nil
	}
	socketPath, _ := strings.CutPrefix(opts.BindAddress, socketPrefix)
	info, err := os.Stat(socketPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Mode().Type() != os.ModeSocket {
		return fmt.Errorf("socket bind path exist, but not a socket")
	}
	if err := os.Remove(socketPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to remove socket: %v", err)
	}
	return nil
}

func deployShimCNI(log logr.Logger, opts *options.Options) error {
	// copy nv-ipam binary
	if !opts.CNISkipBinFileCopy {
		if err := cmdutils.CopyFileAtomic(opts.CNIBinFile, opts.CNIBinDir,
			"_nv-ipam", "nv-ipam"); err != nil {
			log.Error(err, "failed at nv-ipam copy")
			return err
		}
	}
	if !opts.CNISkipConfigCreation {
		return createNVIPAMConfig(log, opts)
	}
	return nil
}

func createNVIPAMConfig(log logr.Logger, opts *options.Options) error {
	cfg := fmt.Sprintf(`{
  "daemonSocket":    "%s",
  "daemonCallTimeoutSeconds":    %d,
  "logFile":   "%s",
  "logLevel": "%s"
}
`, opts.CNIDaemonSocket, opts.CNIDaemonCallTimeoutSeconds, opts.CNILogFile, opts.CNILogLevel)

	err := renameio.WriteFile(filepath.Join(opts.CNIConfDir, cniTypes.ConfFileName), []byte(cfg), 0664)
	if err != nil {
		log.Error(err, "failed to write configuration for shim CNI")
		return err
	}
	log.Info("config for shim CNI written", "config", cfg)
	return nil
}

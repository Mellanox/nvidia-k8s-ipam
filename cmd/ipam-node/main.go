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

package main

import (
	"bytes"
	"crypto/sha256"
	b64 "encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"text/template"
	"time"

	"github.com/spf13/pflag"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cmdutils"
)

// Options stores command line options
type Options struct {
	CNIBinDir                string
	NvIpamCNIBinFile         string
	SkipNvIpamCNIBinaryCopy  bool
	NvIpamCNIDataDir         string
	NvIpamCNIDataDirHost     string
	CNIConfDir               string
	HostLocalBinFile         string // may be hidden or remove?
	SkipHostLocalBinaryCopy  bool
	NvIpamKubeConfigFileHost string
	NvIpamLogLevel           string
	NvIpamLogFile            string
	SkipTLSVerify            bool
}

const (
	serviceAccountTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"  //nolint:golint,gosec
	serviceAccountCAFile    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt" //nolint:golint,gosec
)

func (o *Options) addFlags() {
	// suppress error message for help
	pflag.ErrHelp = nil //nolint:golint,reassign
	fs := pflag.CommandLine
	fs.StringVar(&o.CNIBinDir,
		"cni-bin-dir", "/host/opt/cni/bin", "CNI binary directory")
	fs.StringVar(&o.NvIpamCNIBinFile,
		"nv-ipam-bin-file", "/nv-ipam", "nv-ipam binary file path")
	fs.BoolVar(&o.SkipNvIpamCNIBinaryCopy,
		"skip-nv-ipam-binary-copy", false, "skip nv-ipam binary file copy")
	fs.StringVar(&o.NvIpamCNIDataDir,
		"nv-ipam-cni-data-dir", "/host/var/lib/cni/nv-ipam", "nv-ipam CNI data directory")
	fs.StringVar(&o.NvIpamCNIDataDirHost,
		"nv-ipam-cni-data-dir-host", "/var/lib/cni/nv-ipam", "nv-ipam CNI data directory on host")
	fs.StringVar(&o.CNIConfDir,
		"cni-conf-dir", "/host/etc/cni/net.d", "CNI config directory")
	fs.StringVar(&o.HostLocalBinFile,
		"host-local-bin-file", "/host-local", "host-local binary file path")
	fs.BoolVar(&o.SkipHostLocalBinaryCopy,
		"skip-host-local-binary-copy", false, "skip host-local binary file copy")
	fs.StringVar(&o.NvIpamKubeConfigFileHost,
		"nv-ipam-kubeconfig-file-host", "/etc/cni/net.d/nv-ipam.d/nv-ipam.kubeconfig", "kubeconfig for nv-ipam")
	fs.StringVar(&o.NvIpamLogLevel,
		"nv-ipam-log-level", "info", "nv-ipam log level")
	fs.StringVar(&o.NvIpamLogFile,
		"nv-ipam-log-file", "/var/log/nv-ipam-cni.log", "nv-ipam log file")
	fs.BoolVar(&o.SkipTLSVerify,
		"skip-tls-verify", false, "skip TLS verify")
	fs.MarkHidden("skip-tls-verify") //nolint:golint,errcheck
}

func (o *Options) verifyFileExists() error {
	// CNIBinDir
	if _, err := os.Stat(o.CNIBinDir); err != nil {
		return fmt.Errorf("cni-bin-dir is not found: %v", err)
	}

	// CNIConfDir
	if _, err := os.Stat(o.CNIConfDir); err != nil {
		return fmt.Errorf("cni-conf-dir is not found: %v", err)
	}

	if _, err := os.Stat(fmt.Sprintf("%s/bin", o.NvIpamCNIDataDir)); err != nil {
		return fmt.Errorf("nv-ipam-cni-data-bin-dir is not found: %v", err)
	}

	if _, err := os.Stat(fmt.Sprintf("%s/state/host-local", o.NvIpamCNIDataDir)); err != nil {
		return fmt.Errorf("nv-ipam-cni-data-state-dir is not found: %v", err)
	}

	// HostLocalBinFile
	if _, err := os.Stat(o.HostLocalBinFile); err != nil {
		return fmt.Errorf("host-local-bin-file is not found: %v", err)
	}
	return nil
}

const kubeConfigTemplate = `# Kubeconfig file for nv-ipam CNI plugin.
apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    server: {{ .KubeConfigHost }}
    {{ .KubeServerTLS }}
users:
- name: nv-ipam-node
  user:
    token: "{{ .KubeServiceAccountToken }}"
contexts:
- name: nv-ipam-node-context
  context:
    cluster: local
    user: nv-ipam-node
current-context: nv-ipam-node-context
`

const nvIpamConfigTemplate = `{
    "kubeconfig": "{{ .KubeConfigFile }}",
    "dataDir":    "{{ .NvIpamDataDir }}",
    "logFile":    "{{ .NvIpamLogFile }}",
    "logLevel":   "{{ .NvIpamLogLevel }}"
}
`

func (o *Options) createKubeConfig(currentFileHash []byte) ([]byte, error) {
	// check file exists
	if _, err := os.Stat(serviceAccountTokenFile); err != nil {
		return nil, fmt.Errorf("service account token is not found: %v", err)
	}
	if _, err := os.Stat(serviceAccountCAFile); err != nil {
		return nil, fmt.Errorf("service account ca is not found: %v", err)
	}

	// create nv-ipam.d directory
	if err := os.MkdirAll(fmt.Sprintf("%s/nv-ipam.d", o.CNIConfDir), 0755); err != nil {
		return nil, fmt.Errorf("cannot create nv-ipam.d directory: %v", err)
	}

	// get Kubernetes service protocol/host/port
	kubeProtocol := os.Getenv("KUBERNETES_SERVICE_PROTOCOL")
	if kubeProtocol == "" {
		kubeProtocol = "https"
	}
	kubeHost := os.Getenv("KUBERNETES_SERVICE_HOST")
	kubePort := os.Getenv("KUBERNETES_SERVICE_PORT")

	// check tlsConfig
	var tlsConfig string
	if o.SkipTLSVerify {
		tlsConfig = "insecure-skip-tls-verify: true"
	} else {
		// create tlsConfig by service account CA file
		caFileByte, err := os.ReadFile(serviceAccountCAFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read service account ca file: %v", err)
		}
		caFileB64 := bytes.ReplaceAll([]byte(b64.StdEncoding.EncodeToString(caFileByte)), []byte("\n"), []byte(""))
		tlsConfig = fmt.Sprintf("certificate-authority-data: %s", string(caFileB64))
	}

	saTokenByte, err := os.ReadFile(serviceAccountTokenFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read service account token file: %v", err)
	}

	// create kubeconfig by template and replace it by atomic
	tempKubeConfigFile := fmt.Sprintf("%s/nv-ipam.d/nv-ipam.kubeconfig.new", o.CNIConfDir)
	nvIPAMKubeConfig := fmt.Sprintf("%s/nv-ipam.d/nv-ipam.kubeconfig", o.CNIConfDir)
	fp, err := os.OpenFile(tempKubeConfigFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("cannot create kubeconfig temp file: %v", err)
	}

	templateKubeconfig, err := template.New("kubeconfig").Parse(kubeConfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %v", err)
	}
	templateData := map[string]string{
		"KubeConfigHost":          fmt.Sprintf("%s://[%s]:%s", kubeProtocol, kubeHost, kubePort),
		"KubeServerTLS":           tlsConfig,
		"KubeServiceAccountToken": string(saTokenByte),
	}

	// Prepare
	hash := sha256.New()
	writer := io.MultiWriter(hash, fp)

	// genearate kubeconfig from template
	if err = templateKubeconfig.Execute(writer, templateData); err != nil {
		return nil, fmt.Errorf("cannot create kubeconfig: %v", err)
	}

	if err := fp.Sync(); err != nil {
		os.Remove(fp.Name())
		return nil, fmt.Errorf("cannot flush kubeconfig temp file: %v", err)
	}
	if err := fp.Close(); err != nil {
		os.Remove(fp.Name())
		return nil, fmt.Errorf("cannot close kubeconfig temp file: %v", err)
	}

	newFileHash := hash.Sum(nil)
	if currentFileHash != nil && bytes.Equal(newFileHash, currentFileHash) {
		log.Printf("kubeconfig is same, not copy\n")
		os.Remove(fp.Name())
		return currentFileHash, nil
	}

	// replace file with tempfile
	if err := os.Rename(tempKubeConfigFile, nvIPAMKubeConfig); err != nil {
		return nil, fmt.Errorf("cannot replace %q with temp file %q: %v", nvIPAMKubeConfig, tempKubeConfigFile, err)
	}

	log.Printf("kubeconfig is created in %s\n", nvIPAMKubeConfig)
	return newFileHash, nil
}
func (o *Options) createNvIpamConfig(currentFileHash []byte) ([]byte, error) {
	// create kubeconfig by template and replace it by atomic
	tempNvIpamConfigFile := fmt.Sprintf("%s/nv-ipam.d/nv-ipam.conf.new", o.CNIConfDir)
	nvIpamConfigFile := fmt.Sprintf("%s/nv-ipam.d/nv-ipam.conf", o.CNIConfDir)
	fp, err := os.OpenFile(tempNvIpamConfigFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("cannot create nv-ipam.conf temp file: %v", err)
	}

	templateNvIpamConfig, err := template.New("nv-ipam-config").Parse(nvIpamConfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %v", err)
	}

	templateData := map[string]string{
		"KubeConfigFile": o.NvIpamKubeConfigFileHost,
		"NvIpamDataDir":  o.NvIpamCNIDataDirHost,
		"NvIpamLogFile":  o.NvIpamLogFile,
		"NvIpamLogLevel": o.NvIpamLogLevel,
	}

	// Prepare
	hash := sha256.New()
	writer := io.MultiWriter(hash, fp)

	// genearate nv-ipam-config from template
	if err = templateNvIpamConfig.Execute(writer, templateData); err != nil {
		return nil, fmt.Errorf("cannot create nv-ipam-config: %v", err)
	}

	if err := fp.Sync(); err != nil {
		os.Remove(fp.Name())
		return nil, fmt.Errorf("cannot flush nv-ipam-config temp file: %v", err)
	}
	if err := fp.Close(); err != nil {
		os.Remove(fp.Name())
		return nil, fmt.Errorf("cannot close nv-ipam-config temp file: %v", err)
	}

	newFileHash := hash.Sum(nil)
	if currentFileHash != nil && bytes.Equal(newFileHash, currentFileHash) {
		log.Printf("nv-ipam-config is same, not copy\n")
		os.Remove(fp.Name())
		return currentFileHash, nil
	}

	// replace file with tempfile
	if err := os.Rename(tempNvIpamConfigFile, nvIpamConfigFile); err != nil {
		return nil, fmt.Errorf("cannot replace %q with temp file %q: %v", nvIpamConfigFile, tempNvIpamConfigFile, err)
	}

	log.Printf("nv-ipam-config is created in %s\n", nvIpamConfigFile)
	return newFileHash, nil
}

func main() {
	opt := Options{}
	opt.addFlags()
	helpFlag := pflag.BoolP("help", "h", false, "show help message and quit")

	pflag.Parse()
	if *helpFlag {
		pflag.PrintDefaults()
		os.Exit(1)
	}

	err := opt.verifyFileExists()
	if err != nil {
		log.Printf("%v\n", err)
		return
	}

	// copy nv-ipam binary
	if !opt.SkipNvIpamCNIBinaryCopy {
		// Copy
		if err = cmdutils.CopyFileAtomic(opt.NvIpamCNIBinFile, opt.CNIBinDir, "_nv-ipam", "nv-ipam"); err != nil {
			log.Printf("failed at nv-ipam copy: %v\n", err)
			return
		}
	}

	// copy host-local binary
	if !opt.SkipHostLocalBinaryCopy {
		// Copy
		hostLocalCNIBinDir := fmt.Sprintf("%s/bin", opt.NvIpamCNIDataDir)
		if err = cmdutils.CopyFileAtomic(opt.HostLocalBinFile, hostLocalCNIBinDir, "_host-local", "host-local"); err != nil {
			log.Printf("failed at host-local copy: %v\n", err)
			return
		}
	}

	_, err = opt.createKubeConfig(nil)
	if err != nil {
		log.Printf("failed to create nv-ipam kubeconfig: %v\n", err)
		return
	}
	log.Printf("kubeconfig file is created.\n")

	_, err = opt.createNvIpamConfig(nil)
	if err != nil {
		log.Printf("failed to create nv-ipam config file: %v\n", err)
		return
	}
	log.Printf("nv-ipam config file is created.\n")

	nodeName := os.Getenv("NODE_NAME")
	err = os.WriteFile(fmt.Sprintf("%s/nv-ipam.d/k8s-node-name", opt.CNIConfDir), []byte(nodeName), 0600)
	if err != nil {
		log.Printf("failed to create nv-ipam k8s-node-name: %v\n", err)
		return
	}
	log.Printf("k8s-node-name file is created.\n")

	// sleep infinitely
	for {
		time.Sleep(time.Duration(1<<63 - 1))
	}
}

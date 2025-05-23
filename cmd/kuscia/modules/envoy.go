// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package modules

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	utilscommon "github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/readyz"
	"github.com/secretflow/kuscia/pkg/utils/supervisor"
)

const (
	// CheckTimeInterval is Envoy log files size check interval.
	CheckTimeInterval = "1h"
)

type envoyModule struct {
	moduleRuntimeBase
	rootDir               string
	cluster               string
	id                    string
	commandLineConfigFile string
	logrotate             confloader.LogrotateConfig
}

type EnvoyCommandLineConfig struct {
	Args []string `yaml:"args,omitempty"`
}

func getEnvoyCluster(domain string) string {
	return fmt.Sprintf("kuscia-gateway-%s", domain)
}

func NewEnvoy(i *ModuleRuntimeConfigs) (Module, error) {
	return &envoyModule{
		moduleRuntimeBase: moduleRuntimeBase{
			name:         "envoy",
			readyTimeout: 60 * time.Second,
			rdz: readyz.NewHTTPReadyZ("http://127.0.0.1:10000/ready", 200, func(body []byte) error {
				res := string(body[:len(body)-1])
				if res != "LIVE" {
					return errors.New("response is not live")
				}
				return nil
			}),
		},
		rootDir:               i.RootDir,
		cluster:               getEnvoyCluster(i.DomainID),
		id:                    fmt.Sprintf("%s-%s", getEnvoyCluster(i.DomainID), utils.GetHostname()),
		commandLineConfigFile: "envoy/command-line.yaml",
		logrotate:             i.Logrorate,
	}, nil
}

func (s *envoyModule) Run(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Join(s.rootDir, common.LogPrefix, "envoy/"), 0755); err != nil {
		return err
	}
	deltaArgs, err := s.readCommandArgs()
	if err != nil {
		return err
	}

	args := []string{
		"-c",
		filepath.Join(s.rootDir, common.ConfPrefix, "envoy/envoy.yaml"),
		"--service-cluster",
		s.cluster,
		"--service-node",
		s.id,
		"--log-path",
		filepath.Join(s.rootDir, common.LogPrefix, "envoy/envoy.log"),
	}
	args = append(args, deltaArgs.Args...)
	sp := supervisor.NewSupervisor("envoy", nil, -1)

	configFilePath, err := s.renderLogRotateConfig()
	if err != nil {
		return err
	}

	// start logrotate task
	go s.logRotate(ctx, configFilePath)
	go s.logRotateWithFileSize(ctx, configFilePath)

	return sp.Run(ctx, func(ctx context.Context) supervisor.Cmd {
		cmd := exec.Command(filepath.Join(s.rootDir, "bin/envoy"), args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		return &ModuleCMD{
			cmd:   cmd,
			score: &envoyOOMScore,
		}
	})
}

func (s *envoyModule) renderLogRotateConfig() (configPath string, err error) {

	filePath := filepath.Join(s.rootDir, common.ConfPrefix, "logrotate.conf")
	tmplFilePath := filepath.Join(s.rootDir, common.ConfPrefix, "logrotate.conf.tmpl")

	if err := utilscommon.RenderConfig(tmplFilePath, filePath, s.logrotate); err != nil {
		return "", fmt.Errorf("failed to render logrotate config: %w", err)
	}
	return filePath, nil
}

func (s *envoyModule) logRotate(ctx context.Context, filePath string) {
	for {
		t := time.Now()
		n := time.Date(t.Year(), t.Month(), t.Day(), 0, 1, 0, 0, t.Location())
		d := n.Sub(t)
		if d < 0 {
			n = n.Add(24 * time.Hour)
			d = n.Sub(t)
		}

		select {
		case <-ctx.Done():
			nlog.Warnf("Context done, exit logRotate")
			return
		case <-time.After(d):
		}

		rotate(filePath)

	}
}

func rotate(configFilePath string) {

	cmd := exec.Command("logrotate", configFilePath)
	if err := cmd.Run(); err != nil {
		nlog.Errorf("Logrotate run error: %v", err)
	}
}

func (s *envoyModule) logRotateWithFileSize(ctx context.Context, configFilePath string) {

	// output envoy logrotate config.
	// If the logrotate related configuration items are not configured,
	// the default configuration is automatically loaded in the read kuscia.yaml.
	// confloader/config.go#DefaultKusciaConfig
	nlog.Debugf("envoy logrotate config: %v", s.logrotate)

	duration, err := time.ParseDuration(CheckTimeInterval)
	if err != nil {
		nlog.Errorf("Error while parsing time interval: %v", err)
		return
	}

	// Create a timer to check the log file size every hour
	// default time interval is 1 hour.
	ticker, err := createTicker(duration)
	if err != nil {
		nlog.Error(err)
		return
	}
	defer ticker.Stop()

	checkLogFiles := func() {
		logFiles, err := filepath.Glob(filepath.Join(s.rootDir, common.LogPrefix, "envoy", "*.log"))
		if err != nil {
			nlog.Errorf("Error while globbing log files: %v", err)
			return
		}

		for _, logFile := range logFiles {
			fileSize := getFileSize(logFile)
			if fileSize >= s.logrotate.MaxFileSizeMB {

				nlog.Infof("Log file %s exceeds max size of %dMB, triggering logrotate, check time interval: %s", logFile, s.logrotate.MaxFileSizeMB, CheckTimeInterval)
				rotate(configFilePath)
			}
		}
	}

	for {

		// Wait for timer to fire or context to close
		select {
		case <-ctx.Done():
			nlog.Warnf("Context done, exit logRotate")
			return
		case <-ticker.C:
			checkLogFiles()
		}
	}
}

// getFileSize get file size, return xxx MB
func getFileSize(filePath string) int {

	file, err := os.Stat(filePath)
	if err != nil {
		nlog.Errorf("Error while getting file size: %v", err)
		return 0
	}

	return int(file.Size() / (1024 * 1024))
}

// createTicker Create a timer and check whether the time interval is reasonable
func createTicker(interval time.Duration) (*time.Ticker, error) {

	if interval <= 0 {
		return nil, fmt.Errorf("invalid interval: %v. Interval must be greater than 0", interval)
	}

	return time.NewTicker(interval), nil
}

func (s *envoyModule) readCommandArgs() (*EnvoyCommandLineConfig, error) {
	configPath := filepath.Join(s.rootDir, common.ConfPrefix, s.commandLineConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config EnvoyCommandLineConfig
	err = yaml.Unmarshal(data, &config)
	return &config, err
}

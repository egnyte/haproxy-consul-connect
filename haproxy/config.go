package haproxy

import (
	"crypto/rand"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path"
	"runtime"

	"text/template"

	"github.com/haproxytech/haproxy-consul-connect/lib"
	log "github.com/sirupsen/logrus"
)

const (
	dataplaneUser = "haproxy"
)

var dataplanePass string

var baseCfgTmpl = `
global
	master-worker
    stats socket {{.SocketPath}} mode 600 level admin expose-fd listeners
    stats timeout 2m
	tune.ssl.default-dh-param 1024
	nbproc 1
	nbthread {{.NbThread}}
	log-tag haproxy_sidecar

userlist controller
	user {{.DataplaneUser}} insecure-password {{.DataplanePass}}
`

const spoeConfTmpl = `
[intentions]

spoe-agent intentions-agent
    messages check-intentions

    option var-prefix connect

    timeout hello      3000ms
    timeout idle       3000s
    timeout processing 3000ms

    use-backend spoe_back

spoe-message check-intentions
    args ip=src cert=ssl_c_der
    event on-frontend-tcp-request
`

type baseParams struct {
	NbThread      int
	SocketPath    string
	DataplaneUser string
	DataplanePass string
}

type haConfig struct {
	ConfigsDir              string
	HAProxy                 string
	SPOE                    string
	SPOESock                string
	StatsSock               string
	DataplaneSock           string
	DataplaneTransactionDir string
	LogsSock                string
}

func newHaConfig(baseDir string, sd *lib.Shutdown) (*haConfig, error) {
	cfg := &haConfig{}

	configsDir, err := newTempDirForConfig(baseDir, sd)
	if err != nil {
		return nil, err
	}

	cfg.ConfigsDir = configsDir
	cfg.HAProxy = path.Join(configsDir, "haproxy.conf")
	cfg.SPOE = path.Join(configsDir, "spoe.conf")
	cfg.SPOESock = path.Join(configsDir, "spoe.sock")
	cfg.StatsSock = path.Join(configsDir, "haproxy.sock")
	cfg.DataplaneSock = path.Join(configsDir, "dataplane.sock")
	cfg.DataplaneTransactionDir = path.Join(configsDir, "dataplane-transactions")
	cfg.LogsSock = path.Join(configsDir, "logs.sock")

	err = newHAproxyConfig(cfg, sd)
	if err != nil {
		return nil, err
	}

	err = newSPOEConfig(cfg, sd)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func newTempDirForConfig(baseDir string, sd *lib.Shutdown) (string, error) {

	sd.Add(1)
	tempConfigsDir, err := ioutil.TempDir(baseDir, "haproxy-connect-")
	if err != nil {
		sd.Done()
		return "", err
	}
	go func() {
		defer sd.Done()
		<-sd.Stop
		log.Info("cleaning config...")
		os.RemoveAll(tempConfigsDir)
	}()

	return tempConfigsDir, nil
}

func newHAproxyConfig(cfg *haConfig, sd *lib.Shutdown) error {

	tmpl, err := template.New("cfg").Parse(baseCfgTmpl)
	if err != nil {
		return err
	}

	cfgFile, err := os.OpenFile(cfg.HAProxy, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer cfgFile.Close()

	err = tmpl.Execute(cfgFile, baseParams{
		NbThread:      runtime.GOMAXPROCS(0),
		SocketPath:    cfg.StatsSock,
		DataplaneUser: dataplaneUser,
		DataplanePass: createRandomString(),
	})
	if err != nil {
		sd.Done()
		return err
	}

	return nil
}

func newSPOEConfig(cfg *haConfig, sd *lib.Shutdown) error {

	spoeCfgFile, err := os.OpenFile(cfg.SPOE, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		sd.Done()
		return err
	}
	defer spoeCfgFile.Close()
	_, err = spoeCfgFile.WriteString(spoeConfTmpl)
	if err != nil {
		sd.Done()
		return err
	}

	return nil
}

func createRandomString() string {
	randBytes := make([]byte, 32)
	_, _ = rand.Read(randBytes)
	return base64.URLEncoding.EncodeToString(randBytes)
}

package haproxy

type Options struct {
	HAProxyBin           string
	DataplaneBin         string
	ConfigBaseDir        string
	HaproxyCfgTemplate   string
	SPOEAddress          string
	EnableIntentions     bool
	StatsListenAddr      string
	StatsRegisterService bool
	HAProxyLogRequests   bool
	HAProxyLogAddress    string
}

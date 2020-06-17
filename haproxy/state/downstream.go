package state

import (
	"fmt"

	"github.com/haproxytech/haproxy-consul-connect/consul"
	"github.com/haproxytech/models"
)

func generateDownstream(opts Options, certStore CertificateStore, cfg consul.Downstream, state State) (State, error) {
	feName := "front_downstream"
	beName := "back_downstream"
	feMode := models.FrontendModeHTTP
	beMode := models.BackendModeHTTP

	caPath, crtPath, err := certStore.CertsPath(cfg.TLS)
	if err != nil {
		return state, err
	}

	if cfg.Protocol != "" && cfg.Protocol == "tcp" {
		feMode = models.FrontendModeTCP
		beMode = models.BackendModeTCP
	}

	// Main config
	fe := Frontend{
		Frontend: models.Frontend{
			Name:           feName,
			DefaultBackend: beName,
			ClientTimeout:  &clientTimeout,
			Mode:           feMode,
			Httplog:        opts.LogRequests,
		},
		Bind: models.Bind{
			Name:           fmt.Sprintf("%s_bind", feName),
			Address:        cfg.LocalBindAddress,
			Port:           int64p(cfg.LocalBindPort),
			Ssl:            true,
			SslCertificate: crtPath,
			SslCafile:      caPath,
			Verify:         models.BindVerifyRequired,
		},
	}

	// Logging
	if opts.LogRequests {
		fe.LogTarget = &models.LogTarget{
			ID:       int64p(0),
			Address:  opts.LogAddress,
			Facility: models.LogTargetFacilityLocal0,
			Format:   models.LogTargetFormatRfc5424,
		}
	}

	// Intentions
	if opts.EnableIntentions {
		fe.Filter = &FrontendFilter{
			Filter: models.Filter{
				ID:         int64p(0),
				Type:       models.FilterTypeSpoe,
				SpoeEngine: "intentions",
				SpoeConfig: opts.SPOEConfigPath,
			},
			Rule: models.TCPRequestRule{
				ID:       int64p(0),
				Action:   models.TCPRequestRuleActionReject,
				Cond:     models.TCPRequestRuleCondUnless,
				CondTest: "{ var(sess.connect.auth) -m int eq 1 }",
				Type:     models.TCPRequestRuleTypeContent,
			},
		}
	}

	state.Frontends = append(state.Frontends, fe)

	var forwardFor *models.Forwardfor
	if cfg.EnableForwardFor && beMode == models.BackendModeHTTP {
		forwardFor = &models.Forwardfor{
			Enabled: stringp(models.ForwardforEnabledEnabled),
		}
	}

	// Backend
	be := Backend{
		Backend: models.Backend{
			Name:           beName,
			ServerTimeout:  &serverTimeout,
			ConnectTimeout: &connectTimeout,
			Mode:           beMode,
			Forwardfor:     forwardFor,
		},
		Servers: []models.Server{
			models.Server{
				Name:    "downstream_node",
				Address: cfg.TargetAddress,
				Port:    int64p(cfg.TargetPort),
			},
		},
	}

	// Logging
	if opts.LogRequests {
		be.LogTarget = &models.LogTarget{
			ID:       int64p(0),
			Address:  opts.LogAddress,
			Facility: models.LogTargetFacilityLocal0,
			Format:   models.LogTargetFormatRfc5424,
		}
	}

	// App name header
	if cfg.AppNameHeaderName != "" && beMode == models.BackendModeHTTP {
		be.HTTPRequestRules = append(be.HTTPRequestRules, models.HTTPRequestRule{
			ID:        int64p(0),
			Type:      models.HTTPRequestRuleTypeAddHeader,
			HdrName:   cfg.AppNameHeaderName,
			HdrFormat: "%[var(sess.connect.source_app)]",
		})
	}

	state.Backends = append(state.Backends, be)

	return state, nil
}

package soliddns

import (
	"context"
	"crypto/tls"
	"net/http"
	"sigs.k8s.io/external-dns/endpoint"
	"strconv"

	eip "github.com/efficientip-labs/solidserver-go-client/sdsclient"
)

type EfficientIPConfig struct {
	Host       string `env:"EIP_HOST,required" envDefault:"localhost"`
	Port       int    `env:"EIP_PORT,required" envDefault:"443"`
	Username   string `env:"EIP_USER" envDefault:"ipmadmin"`
	Password   string `env:"EIP_PASSWORD" envDefault:""`
	Token      string `env:"EIP_TOKEN" envDefault:""`
	Secret     string `env:"EIP_SECRET" envDefault:""`
	DnsSmart   string `env:"EIP_SMART,required"`
	DnsView    string `env:"EIP_VIEW" envDefault:""`
	SSLVerify  bool   `env:"EIP_SSL_VERIFY" envDefault:"true"`
	DryRun     bool   `env:"EIP_DRY_RUN" envDefault:"false"`
	MaxResults int    `env:"EIP_MAX_RESULTS" envDefault:"1500"`
	CreatePTR  bool   `env:"EIP_CREATE_PTR" envDefault:"false"`
	DefaultTTL int    `env:"EIP_DEFAULT_TTL" envDefault:"300"`
	FQDNRegEx  string
	NameRegEx  string
}

func NewEfficientIPProvider(config *EfficientIPConfig, domainFilter endpoint.DomainFilter) (*Provider, error) {
	clientConfig := eip.NewConfiguration()
	if !config.SSLVerify {
		customTransport := http.DefaultTransport.(*http.Transport).Clone()
		customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		clientConfig.HTTPClient = &http.Client{Transport: customTransport}
	}

	var ctx context.Context
	if config.Token != "" && config.Secret != "" {
		ctx = context.WithValue(context.Background(), eip.ContextEipApiTokenAuth, eip.EipApiTokenAuth{
			Token:  config.Token,
			Secret: config.Secret,
		})
	} else {
		ctx = context.WithValue(context.Background(), eip.ContextBasicAuth, eip.BasicAuth{
			UserName: config.Username,
			Password: config.Password,
		})
	}

	ctx = context.WithValue(ctx, eip.ContextServerVariables, map[string]string{
		"host": config.Host,
		"port": strconv.Itoa(config.Port),
	})
	client := NewEfficientIPAPI(ctx, clientConfig, config)

	return &Provider{
		client:       &client,
		domainFilter: domainFilter,
		context:      ctx,
		config:       config,
	}, nil
}

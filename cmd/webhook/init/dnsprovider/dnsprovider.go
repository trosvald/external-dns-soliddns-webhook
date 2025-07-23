package dnsprovider

import (
	"fmt"
	"github.com/caarlos0/env/v11"
	"regexp"
	"strings"

	"github.com/trosvald/external-dns-soliddns-webhook/cmd/webhook/init/configuration"
	"github.com/trosvald/external-dns-soliddns-webhook/internal/soliddns"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/provider"

	log "github.com/sirupsen/logrus"
)

func Init(config configuration.Config) (provider.Provider, error) {
	var domainFilter endpoint.DomainFilter
	createMsg := "[INFO] Creating EfficientIP SolidDNS provider with "

	if config.RegexDomainFilter != "" {
		createMsg += fmt.Sprintf("regex domain filter: %s", config.RegexDomainFilter)
		if config.RegexDomainExclusion != "" {
			createMsg += fmt.Sprintf(" and regex domain exclusion: %s", config.RegexDomainExclusion)
		}
		domainFilter = endpoint.NewRegexDomainFilter(
			regexp.MustCompile(config.RegexDomainFilter),
			regexp.MustCompile(config.RegexDomainExclusion))
	} else {
		if config.DomainFilter != nil && len(config.DomainFilter) > 0 {
			createMsg += fmt.Sprintf(" domain filter: %s", strings.Join(config.DomainFilter, ","))
		}
		if config.ExcludeDomains != nil && len(config.ExcludeDomains) > 0 {
			createMsg += fmt.Sprintf(" exclude domain filter: %s", strings.Join(config.ExcludeDomains, ","))
		}
		domainFilter = endpoint.NewDomainFilterWithExclusions(config.DomainFilter, config.ExcludeDomains)
	}

	createMsg = strings.TrimSuffix(createMsg, ", ")
	if strings.HasSuffix(createMsg, "with ") {
		createMsg += "no kind of domain filters"
	}
	log.Info(createMsg)

	eipConfig := soliddns.EfficientIPConfig{}
	if err := env.Parse(&eipConfig); err != nil {
		return nil, fmt.Errorf("reading configuration failed: %v", err)
	} else {
		if eipConfig.Token == "" || eipConfig.Secret == "" {
			if eipConfig.Username == "" || eipConfig.Password == "" {
				return nil, fmt.Errorf("missing authentication credentials. Login/Password or access token/secret are required")
			}
		}
	}

	eipConfig.FQDNRegEx = config.RegexDomainFilter
	eipConfig.NameRegEx = config.RegexNameFilter

	return soliddns.NewEfficientIPProvider(&eipConfig, domainFilter)
}

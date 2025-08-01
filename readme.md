# ExternalDNS Webhook Provider for EfficientIP SolidDNS

[//]: # (<div align="center">)

[//]: # ()
[//]: # ([![GitHub Release]&#40;https://img.shields.io/github/v/release/kashalls/external-dns-unifi-webhook?style=for-the-badge&#41;]&#40;https://github.com/kashalls/external-dns-unifi-webhook/releases&#41;&nbsp;&nbsp;)

[//]: # ()
[//]: # (</div>)

[ExternalDNS](https://github.com/kubernetes-sigs/external-dns) is a Kubernetes add-on for automatically managing DNS records for Kubernetes ingresses and services by using different DNS providers. This webhook provider allows you to automate DNS records from your Kubernetes clusters into your EfficientIP SolidDNS appliance.

## 🎯 Requirements

- ExternalDNS >= v0.14.0

## 🚫 Limitations

- WIP

## ⛵ Deployment

### Gathering your credentials

WIP Steps 

```yaml
---
apiVersion: v1
kind: Secret
metadata:
    name: external-dns-eip-secret
stringData:
  token: <your-api-token>
  secret: <your-api-secret>
```


### Installing the provider

1. Add the ExternalDNS Helm repository to your cluster.

    ```sh
    helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
    ```

2. Deploy your `external-dns-soliddns-secret` secret that holds your authentication credentials from either of the credential types above.

3. Create the helm values file, for example `external-dns-soliddns-values.yaml`:

4. Install the Helm chart

    ```sh
    helm install external-dns-soliddns external-dns/external-dns -f external-dns-unifi-values.yaml --version 1.15.0 -n external-dns
    ```

## Configuration

### EfficientIP SolidDNS Controller Configuration

| Environment Variable   | Default value | Required |
|------------------------|---------------|----------|
| EIP_HOST               | localhost     | true     |
| EIP_PORT               | 443           | true     |   
| EIP_USER               |               | false    |
| EIP_PASSWORD           |               | false    |
| EIP_TOKEN              |               | false    |
| EIP_SECRET             |               | false    |
| EIP_SMART              |               | true     |
| EIP_VIEW               |               | false    |
| EIP_SSL_VERIFY         | true          | false    |
| EIP_DRY_RUN            | false         | false    |
| EIP_DEFAULT_TTL        | 300           | false    |

### Server Configuration

| Environment Variable           | Default value | Required |
|--------------------------------|---------------|----------|
| SERVER_HOST                    | 0.0.0.0       | true     |
| SERVER_PORT                    | 8888          | true     |   
| SERVER_READ_TIMEOUT            |               | false    |
| SERVER_WRITE_TIMEOUT           |               | false    |
| DOMAIN_FILTER                  |               | false    |
| EXCLUDE_DOMAIN_FILTER          |               | false    |
| REGEXP_DOMAIN_FILTER           |               | false    |
| REGEXP_DOMAIN_FILTER_EXCLUSION |               | false    |
| REGEXP_NAME_FILTER             |               | false    |

## Running locally

To run provider in a local environment, you must provide all required settings through environment variables.
To run locally, set `SERVER_HOST` to `localhost`, otherwise leave it at `0.0.0.0`.
EfficientIP Provider is a simple web server with several clearly defined routers:

| Route            | Method |
|------------------|--------|
| /healthz         | GET    |
| /records         | GET    |
| /records         | POST   |
| /adjustendpoints | POST   |

#### Reading Data
Read data by HTTP GET to `/records`, see:
```shell
curl -H 'Accept: application/external.dns.webhook+json;version=1' localhost:8888/records
```
If you set DOMAIN_FILTER, DNS will return all records from this domain(s). Because the returned data for a given
domain can be large - in some cases tens of thousands of records, it is advisable to use filters to reduce the
data to the desired result. Filters are specified via environment variables: `DOMAIN_FILTER`,`EXCLUDE_DOMAIN_FILTER`,
`REGEXP_DOMAIN_FILTER`,`REGEXP_DOMAIN_FILTER_EXCLUSION`,`REGEXP_NAME_FILTER`.

The following example demonstrates the use of a filter:
```shell
# We are looking for all records in these two domains. 
# Unfortunately, they may contain tens of thousands of records.
DOMAIN_FILTER=org.eu.cloud.example.com,org-hq.us.cloud.example.com

# If DOMAIN_FILTER is not enough, you can use regex. Once you use REGEXP_DOMAIN_FILTER, DOMAIN_FILTER will be ignored.
# In following example we restrict zones to *.eu.cloud.example.com or *.org-hq.us.cloud.example.com.
REGEXP_DOMAIN_FILTER=(eu.cloud|org-hq.us).cloud.example.com

# Finally, we filter only those records that have `my-project.org-hq` or `.us.cloud` in the name
REGEXP_NAME_FILTER=(my-project.org-hq|.us.cloud)
```

#### Writing Data

Here are the updating rules according to which the data in the DNS server will be updated:

- if updateNew is not part of Update Old , object should be created
- if updateOld is not part of Update New , object should be deleted
- if information is not present (TTL might change) , object should be updated
- if we rename the object, object should be deleted and created


Based on the rules I am providing some examples of `data.json` creating, changing and deleting records in DNS.

```shell
curl -X POST -H 'Accept: application/external.dns.webhook+json;version=1;' -H 'Content-Type: application/external.dns.webhook+json;version=1' -d @data.json localhost:8888/records
```

Create `test.cloud.example.com`
```json
{"Create":null,"UpdateOld":null,"UpdateNew":[{"dnsName":"test.cloud.example.com","targets":["1.3.2.1"],"recordType":"A","recordTTL":300}],"Delete":null}
```

Update `test.cloud.example.com` (DELETE one record `test.cloud.example.com` and CREATE two records `new-test.cloud.example.com`)
```json
{"Create":null,"UpdateOld":[{"dnsName":"test.cloud.example.com","targets":["1.3.2.1"],"recordType":"A","recordTTL":300}],"UpdateNew":[{"dnsName":"new-test.cloud.example.com","targets":["1.2.3.4","4.3.2.1"],"recordType":"A","recordTTL":300}],"Delete":null}
```

Delete `test-new.cloud.example.com`
```json
{"Create":null,"UpdateOld":[{"dnsName":"new-test.cloud.example.","targets":["1.2.3.4","4.3.2.1"],"recordType":"A","recordTTL":300}],"UpdateNew":null,"Delete":null}
```
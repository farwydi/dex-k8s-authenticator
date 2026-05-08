# Configuration

An example configuration is available [here](../examples/config.yaml)

## Configuration options


| Name                      | Required | Context | Description                                                                           |
|---------------------------|----------|---------|---------------------------------------------------------------------------------------|
| name                      | yes      | cluster | Internal id of cluster                                                                |
| short_description         | yes      | cluster | Short description of cluster                                                          |
| description               | yes      | cluster | Extended description of cluster                                                       |
| client_secret             | no       | cluster | OAuth2 client-secret (shared between dex-k8s-auth and dex). Omit when using a public client with PKCE. |
| client_id                 | yes      | cluster | OAuth2 client-id public identifier (shared between dex-k8s-auth and dex)              |
| client_type               | no       | cluster | OAuth2 client type: `confidential` (default) or `public`. Setting `public` enables PKCE and makes `client_secret` optional. |
| pkce                      | no       | cluster | Enable PKCE (RFC 7636, S256) on the authorization-code flow. Auto-enabled when `client_type: public`. |
| connector_id              | no       | cluster | Dex connector ID to use by default omitting other available connectors                |
| issuer                    | yes      | cluster | Dex issuer url                                                                        |
| redirect_uri              | yes      | cluster | Redirect uri called by dex (defines a callback on dex-k8s-auth)                       |
| k8s_master_uri            | no       | cluster | Kubernetes api-server endpoint (used in kubeconfig)                                   |
| k8s_ca_uri                | no       | cluster | A url pointing to the CA for generating 'certificate-authority' option in kubeconfig  |
| k8s_ca_pem                | no       | cluster | The CA for your k8s server (used in generating instructions)                          |
| k8s_ca_pem_base64_encoded | no       | cluster | The Base64 encoded CA for your k8s server (used in generating instructions)           |
| k8s_ca_pem_file           | no       | cluster | The CA file for your k8s server (used in generating instructions)                     |
| scopes                    | no       | cluster | A list OpenID scopes to request                                                       |
| tls_cert                  | no       | root    | Path to TLS cert if SSL enabled                                                       |
| tls_key                   | no       | root    | Path to TLS key if SSL enabled                                                        |
| idp_ca_uri                | no       | root    | A url pointing to the CA for generating 'idp-certificate-authority' in the kubeconfig |
| idp_ca_pem                | no       | root    | The CA for generating 'idp-certificate-authority' in the kubeconfig                   |
| idp_ca_pem_file           | no       | root    | The CA for generating 'idp-certificate-authority' in the kubeconfig - load from file  |
| trusted_root_ca           | no       | root    | A list of trusted-root CA's to be loaded by dex-k8s-auth at runtime                   |
| trusted_root_ca_file      | no       | root    | A list of trusted-root CA's to be loaded by dex-k8s-auth at runtime - load from file  |
| listen                    | yes      | root    | The listen address/port                                                               |
| web_path_prefix           | no       | root    | A path-prefix to serve dex-k8s-auth at (defaults to '/')                              |
| kubectl_version           | no       | root    | A kubectl-version string that is used to provided a download path                     |
| logo_uri                  | no       | root    | A url pointing to a logo image that is displayed in the header                        |
| debug                     | no       | root    | Enable more debug by setting to true                                                  |

## Environment Variable Support

Environment variables can be included in the configuration. This enables you to pass in dynamic variables or secrets without having to hardcode them.
```
clusters:
  - name: example-cluster
    short_description: "Example Cluster"
    description: "Example Cluster Long Description..."
    client_secret: ${CLIENT_SECRET_EXAMPLE_CLUSTER}
```

In this example, `${CLIENT_SECRET_EXAMPLE_CLUSTER}` is substituted at runtime. Must be enclosed by `${...}`

## Web Path Prefix

A common practise is configure `dex-k8s-authenticator` to serve requests under a specific path prefix, such as `https://mycompany.example.com/dex-auth`

You can achieve this by configuring the `web_path_prefix`. In most cases you will need to adjust the Ingress `path` setting to match.

In addition to this, you need to update the `redirect_uri` value.

```yaml
clusters:
  - name: example-cluster
    short_description: "Example Cluster"
    description: "Example Cluster Long Description..."
    redirect_uri: http://127.0.0.1:5555/dex-auth/callback/example-cluster
    client_secret: ...
    client_id: example-cluster-client-id
    issuer:  http://127.0.0.1:5556
    k8s_master_uri: https://your-k8s-master.cluster
    scopes:
      - email
      - profile
      - openid

# A path-prefix from which to serve requests and assets
web_path_prefix: /dex-auth
```

Don't forget to update the Dex `staticClients.redirectURIs` value to include the prefix as well.

## Public clients (PKCE)

Modern IdPs such as authentik or strict Keycloak realms favour public OAuth2
clients combined with PKCE (RFC 7636, S256) instead of distributing a
`client_secret`. To configure dex-k8s-authenticator as a public client,
omit `client_secret` and set `client_type: public` (or `pkce: true`):

```yaml
clusters:
  - name: example-cluster
    short_description: "Example Cluster"
    description: "Example Cluster Long Description..."
    client_id: kubernetes
    client_type: public
    issuer: https://auth.example.com/application/o/kubernetes/
    redirect_uri: https://kubeconfig.example.com/callback/example-cluster
    scopes:
      - openid
      - email
      - profile
      - offline_access
```

Confidential clients with `client_secret` continue to work without changes.
The startup will fail fast if `client_secret` is empty and PKCE is not
enabled.

### Helm

The `dex-k8s-authenticator` helm charts support this via the `dexK8sAuthenticator.web_path_prefix` and `ingress.path` options. You typically set these to the same value.

Note that the health-checks are configured automatically.

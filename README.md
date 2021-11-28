# envoy-router

This repo demonstrates how to use Envoy's [External Authorization](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_authz_filter) aka `ext_authz` filter to dynamically route services

## How to use the filter

This repo relies on two Envoy filters:

* [External Authorization](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_authz_filter) filter
* [Dynamic Forward Proxy](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/dynamic_forward_proxy_filter) filter

The `ext_authz` filter need not be used only for authorizing requests to upstream clusters. In this example, it is being used to map a basePath prefix to an appropriate upstream. 

The `ext_authz` implementation relies on a routing table (in json):

```json
{
    "routerules" : [
      {
        "name": "mocktarget",
        "prefix": "/iloveapis",
        "backend": "mocktarget.apigee.net"
      }   
    ]
}
```

___

## Support

This is not an officially supported Google product

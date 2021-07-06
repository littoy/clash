<h1 align="center">
  <img src="https://github.com/Dreamacro/clash/raw/master/docs/logo.png" alt="Clash" width="200">
  <br>Clash<br>
</h1>

<h4 align="center">A rule-based tunnel in Go.</h4>

<p align="center">
  <a href="https://github.com/Dreamacro/clash/actions">
    <img src="https://img.shields.io/github/workflow/status/Dreamacro/clash/Go?style=flat-square" alt="Github Actions">
  </a>
  <a href="https://goreportcard.com/report/github.com/Dreamacro/clash">
    <img src="https://goreportcard.com/badge/github.com/Dreamacro/clash?style=flat-square">
  </a>
  <a href="https://github.com/Dreamacro/clash/releases">
    <img src="https://img.shields.io/github/release/Dreamacro/clash/all.svg?style=flat-square">
  </a>
</p>

## Features

- Local HTTP/HTTPS/SOCKS server with authentication support
- VMess, Shadowsocks, Trojan, Snell protocol support for remote connections
- Built-in DNS server that aims to minimize DNS pollution attack impact, supports DoH/DoT upstream and fake IP.
- Rules based off domains, GEOIP, IP CIDR or ports to forward packets to different nodes
- Remote groups allow users to implement powerful rules. Supports automatic fallback, load balancing or auto select node based off latency
- Remote providers, allowing users to get node lists remotely instead of hardcoding in config
- Netfilter TCP redirecting. Deploy Clash on your Internet gateway with `iptables`.
- Comprehensive HTTP RESTful API controller

## Premium Features

- TUN mode on macOS, Linux and Windows. [Doc](https://github.com/Dreamacro/clash/wiki/premium-core-features#tun-device)
- Match your tunnel by [Script](https://github.com/Dreamacro/clash/wiki/premium-core-features#script)
- [Rule Provider](https://github.com/Dreamacro/clash/wiki/premium-core-features#rule-providers)

## Getting Started
Documentations are now moved to [GitHub Wiki](https://github.com/Dreamacro/clash/wiki).

## Advanced usage for this branch
### Rules configuration
- Support rule `GEOSITE`
- Support `multiport` condition for rule `SRC-PORT` and `DST-PORT`
- Support not match condition for rule `GEOIP`
- Support `network` condition for all rules

The `GEOSITE` and `GEOIP` databases via https://github.com/Loyalsoldier/v2ray-rules-dat
```yaml
rules:
  # network condition for rules
  - DOMAIN-SUFFIX,bilibili.com,DIRECT,tcp
  - DOMAIN-SUFFIX,bilibili.com,REJECT,udp
    
  # multiport condition for rule SRC-PORT and DST-PORT
  - DST-PORT,123/136/137-139,DIRECT,udp
  
  # rule GEOSITE
  - GEOSITE,category-ads-all,REJECT
  - GEOSITE,icloud@cn,DIRECT
  - GEOSITE,apple@cn,DIRECT
  - GEOSITE,microsoft@cn,DIRECT
  - GEOSITE,facebook,PROXY
  - GEOSITE,youtube,PROXY
  - GEOSITE,geolocation-cn,DIRECT
  - GEOSITE,gfw,PROXY
  - GEOSITE,greatfire,PROXY
  #- GEOSITE,geolocation-!cn,PROXY

  - GEOIP,telegram,PROXY,no-resolve
  - GEOIP,private,DIRECT,no-resolve
  - GEOIP,cn,DIRECT
    
  # Not match condition for rule GEOIP
  #- GEOIP,!cn,PROXY

  - MATCH,PROXY
```

### Display Process name
Add field `Process` to `Metadata` and prepare to get process name for Restful API `GET /connections`

To display process name in GUI please use https://yaling888.github.io/yacd/

## Premium Release
[Release](https://github.com/Dreamacro/clash/releases/tag/premium)

## Development
If you want to build an application that uses clash as a library, check out the the [GitHub Wiki](https://github.com/Dreamacro/clash/wiki/use-clash-as-a-library)

## Credits

* [riobard/go-shadowsocks2](https://github.com/riobard/go-shadowsocks2)
* [v2ray/v2ray-core](https://github.com/v2ray/v2ray-core)

## License

This software is released under the GPL-3.0 license.

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2FDreamacro%2Fclash.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2FDreamacro%2Fclash?ref=badge_large)

## TODO

- [x] Complementing the necessary rule operators
- [x] Redir proxy
- [x] UDP support
- [x] Connection manager

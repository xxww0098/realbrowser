#![forbid(unsafe_code)]

use serde::{Deserialize, Serialize};
use thiserror::Error;
use url::Host;

pub const NETWORK_SCHEMA_VERSION: u16 = 1;

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum NetworkMode {
    Direct,
    Proxy,
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum ProxyProtocol {
    Http,
    Https,
    Socks5,
}

impl ProxyProtocol {
    #[must_use]
    pub const fn scheme(self) -> &'static str {
        match self {
            Self::Http => "http",
            Self::Https => "https",
            Self::Socks5 => "socks5",
        }
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ProxyConfig {
    pub protocol: ProxyProtocol,
    pub host: String,
    pub port: u16,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkConfig {
    pub schema_version: u16,
    pub mode: NetworkMode,
    pub proxy: Option<ProxyConfig>,
}

impl NetworkConfig {
    #[must_use]
    pub const fn direct() -> Self {
        Self {
            schema_version: NETWORK_SCHEMA_VERSION,
            mode: NetworkMode::Direct,
            proxy: None,
        }
    }

    pub fn normalized(self) -> Result<Self, NetworkError> {
        let mut issues = Vec::new();
        if self.schema_version != NETWORK_SCHEMA_VERSION {
            issues.push(NetworkIssue {
                field: "network.schemaVersion",
                code: "unsupported_schema",
                message: "网络配置版本不受支持".to_owned(),
            });
        }

        let proxy = match self.mode {
            NetworkMode::Direct => None,
            NetworkMode::Proxy => self
                .proxy
                .map(|proxy| normalize_proxy(proxy, &mut issues))
                .or_else(|| {
                    issues.push(NetworkIssue {
                        field: "network.proxy",
                        code: "required",
                        message: "请填写代理服务器".to_owned(),
                    });
                    None
                }),
        };

        if issues.is_empty() {
            Ok(Self {
                schema_version: NETWORK_SCHEMA_VERSION,
                mode: self.mode,
                proxy,
            })
        } else {
            Err(NetworkError::Invalid(issues))
        }
    }

    pub fn compile_chromium(&self) -> Result<NetworkPlan, NetworkError> {
        let normalized = self.clone().normalized()?;
        let arguments = match normalized.mode {
            NetworkMode::Direct => Vec::new(),
            NetworkMode::Proxy => {
                let proxy = normalized
                    .proxy
                    .as_ref()
                    .expect("normalized proxy is present");
                vec![
                    format!("--proxy-server={}", render_proxy(proxy)),
                    "--disable-quic".to_owned(),
                ]
            }
        };
        Ok(NetworkPlan {
            mode: normalized.mode,
            arguments,
        })
    }
}

impl Default for NetworkConfig {
    fn default() -> Self {
        Self::direct()
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NetworkPlan {
    pub mode: NetworkMode,
    pub arguments: Vec<String>,
}

impl NetworkPlan {
    #[must_use]
    pub fn rendered_arguments(&self) -> Vec<String> {
        self.arguments.clone()
    }
}

#[derive(Clone, Debug, Eq, Error, PartialEq)]
pub enum NetworkError {
    #[error("网络配置需要修正")]
    Invalid(Vec<NetworkIssue>),
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NetworkIssue {
    pub field: &'static str,
    pub code: &'static str,
    pub message: String,
}

fn normalize_proxy(proxy: ProxyConfig, issues: &mut Vec<NetworkIssue>) -> ProxyConfig {
    let host = proxy.host.trim();
    let parsed = if host.is_empty() {
        issues.push(NetworkIssue {
            field: "network.proxy.host",
            code: "required",
            message: "请输入代理主机".to_owned(),
        });
        None
    } else if host.contains("://") || host.contains('/') || host.contains('@') {
        issues.push(NetworkIssue {
            field: "network.proxy.host",
            code: "host_only",
            message: "主机中不要填写协议、路径或账号".to_owned(),
        });
        None
    } else {
        match Host::parse(host) {
            Ok(host) => Some(host),
            Err(_) => {
                issues.push(NetworkIssue {
                    field: "network.proxy.host",
                    code: "invalid_host",
                    message: "代理主机格式无效".to_owned(),
                });
                None
            }
        }
    };
    if proxy.port == 0 {
        issues.push(NetworkIssue {
            field: "network.proxy.port",
            code: "invalid_port",
            message: "端口必须在 1–65535 之间".to_owned(),
        });
    }
    ProxyConfig {
        protocol: proxy.protocol,
        host: parsed.map_or_else(|| host.to_owned(), canonical_host),
        port: proxy.port,
    }
}

fn canonical_host(host: Host<String>) -> String {
    match host {
        Host::Domain(domain) => domain,
        Host::Ipv4(address) => address.to_string(),
        Host::Ipv6(address) => address.to_string(),
    }
}

fn render_proxy(proxy: &ProxyConfig) -> String {
    let host = if proxy.host.contains(':') {
        format!("[{}]", proxy.host)
    } else {
        proxy.host.clone()
    };
    format!("{}://{}:{}", proxy.protocol.scheme(), host, proxy.port)
}

#[cfg(test)]
mod tests {
    use super::{NetworkConfig, NetworkError, NetworkMode, ProxyConfig, ProxyProtocol};

    #[test]
    fn direct_has_no_chrome_arguments() {
        let plan = NetworkConfig::direct().compile_chromium().unwrap();
        assert_eq!(plan.mode, NetworkMode::Direct);
        assert!(plan.arguments.is_empty());
    }

    #[test]
    fn normalizes_and_compiles_socks5_proxy() {
        let config = NetworkConfig {
            schema_version: 1,
            mode: NetworkMode::Proxy,
            proxy: Some(ProxyConfig {
                protocol: ProxyProtocol::Socks5,
                host: "  Proxy.Example  ".to_owned(),
                port: 10_800,
            }),
        }
        .normalized()
        .unwrap();

        assert_eq!(config.proxy.as_ref().unwrap().host, "proxy.example");
        assert_eq!(
            config.compile_chromium().unwrap().arguments,
            [
                "--proxy-server=socks5://proxy.example:10800",
                "--disable-quic"
            ]
        );
    }

    #[test]
    fn rejects_credentials_or_scheme_in_host() {
        let error = NetworkConfig {
            schema_version: 1,
            mode: NetworkMode::Proxy,
            proxy: Some(ProxyConfig {
                protocol: ProxyProtocol::Http,
                host: "http://user:pass@proxy.example".to_owned(),
                port: 8080,
            }),
        }
        .normalized()
        .unwrap_err();

        assert!(matches!(
            error,
            NetworkError::Invalid(issues) if issues[0].field == "network.proxy.host"
        ));
    }

    #[test]
    fn direct_normalization_discards_stale_proxy_values() {
        let config = NetworkConfig {
            schema_version: 1,
            mode: NetworkMode::Direct,
            proxy: Some(ProxyConfig {
                protocol: ProxyProtocol::Http,
                host: "proxy.example".to_owned(),
                port: 8080,
            }),
        }
        .normalized()
        .unwrap();

        assert_eq!(config, NetworkConfig::direct());
    }
}

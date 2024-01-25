config_version: 2

name: "haproxy-dataplanapi-with-pod-proxier"

mode: "single"

dataplaneapi:
  advertised: {}

haproxy:
  reload:
    reload_strategy: "custom"


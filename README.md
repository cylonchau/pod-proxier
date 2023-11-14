# Pod proxier

Pod 代理，提供一个单Pod代理的入口，通过公共地址可以访问一个Pod，并且可以切换哪个Pod可以访问，用于单Pod调试

```bash
docker run -d --network host --name my-running-haproxy --expose 5555 -p 5555:5555 -p 8404:8404 -v /root/ha:/usr/local/etc/haproxy:rw haproxytech/haproxy-debian:2.6
```
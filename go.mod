module pod-proxier

go 1.22.0

toolchain go1.24.2

replace k8s.io/api => k8s.io/api v0.30.14

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.30.14

replace k8s.io/apimachinery => k8s.io/apimachinery v0.30.14

replace k8s.io/apiserver => k8s.io/apiserver v0.30.14

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.30.14

replace k8s.io/client-go => k8s.io/client-go v0.30.14

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.30.14

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.30.14

replace k8s.io/code-generator => k8s.io/code-generator v0.30.14

replace k8s.io/component-base => k8s.io/component-base v0.30.14

replace k8s.io/controller-manager => k8s.io/controller-manager v0.30.14

replace k8s.io/cri-api => k8s.io/cri-api v0.30.14

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.30.14

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.30.14

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.30.14

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.30.14

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.30.14

replace k8s.io/kubectl => k8s.io/kubectl v0.30.14

replace k8s.io/kubelet => k8s.io/kubelet v0.30.14

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.30.14

replace k8s.io/metrics => k8s.io/metrics v0.30.14

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.30.14

replace k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.30.14

replace k8s.io/sample-controller => k8s.io/sample-controller v0.30.14

require (
	github.com/amit7itz/goset v1.2.1
	github.com/common-nighthawk/go-figure v0.0.0-20210622060536-734e95fb86be
	github.com/cylonchau/gorest v0.4.0
	github.com/gin-gonic/gin v1.9.1
	github.com/haproxytech/models v1.2.4
	github.com/json-iterator/go v1.1.12
	github.com/shirou/gopsutil/v3 v3.23.10
	github.com/spf13/cobra v1.8.0
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.30.14
	k8s.io/apimachinery v0.30.14
	k8s.io/client-go v0.30.14
	k8s.io/klog/v2 v2.120.1
)

require (
	github.com/asaskevich/govalidator v0.0.0-20190424111038-f61b66f89f4a // indirect
	github.com/bytedance/sonic v1.9.1 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/analysis v0.19.5 // indirect
	github.com/go-openapi/errors v0.19.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/loads v0.19.4 // indirect
	github.com/go-openapi/runtime v0.19.4 // indirect
	github.com/go-openapi/spec v0.19.3 // indirect
	github.com/go-openapi/strfmt v0.19.3 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-openapi/validate v0.19.5 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.14.0 // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.4 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mitchellh/mapstructure v1.1.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pelletier/go-toml/v2 v2.0.8 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	go.mongodb.org/mongo-driver v1.1.2 // indirect
	golang.org/x/arch v0.3.0 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/net v0.23.0 // indirect
	golang.org/x/oauth2 v0.10.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/term v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 // indirect
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace k8s.io/component-helpers => k8s.io/component-helpers v0.30.14

replace k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.30.14

replace k8s.io/endpointslice => k8s.io/endpointslice v0.30.14

replace k8s.io/kms => k8s.io/kms v0.30.14

replace k8s.io/mount-utils => k8s.io/mount-utils v0.30.14

replace k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.30.14

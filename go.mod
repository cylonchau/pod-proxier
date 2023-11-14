module pod-proxier

go 1.21.1

replace k8s.io/api => k8s.io/api v0.19.10

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.10

replace k8s.io/apimachinery => k8s.io/apimachinery v0.19.11-rc.0

replace k8s.io/apiserver => k8s.io/apiserver v0.19.10

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.10

replace k8s.io/client-go => k8s.io/client-go v0.19.10

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.10

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.19.10

replace k8s.io/code-generator => k8s.io/code-generator v0.19.13-rc.0

replace k8s.io/component-base => k8s.io/component-base v0.19.10

replace k8s.io/controller-manager => k8s.io/controller-manager v0.19.17-rc.0

replace k8s.io/cri-api => k8s.io/cri-api v0.19.13-rc.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.19.10

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.10

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.19.10

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.19.10

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.19.10

replace k8s.io/kubectl => k8s.io/kubectl v0.19.10

replace k8s.io/kubelet => k8s.io/kubelet v0.19.10

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.19.10

replace k8s.io/metrics => k8s.io/metrics v0.19.10

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.19.10

replace k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.19.10

replace k8s.io/sample-controller => k8s.io/sample-controller v0.19.10

require (
	github.com/amit7itz/goset v1.2.1
	github.com/cylonchau/gorest v0.4.0
	github.com/gin-gonic/gin v1.9.1
	github.com/haproxytech/models v1.2.4
	github.com/json-iterator/go v1.1.12
	github.com/shirou/gopsutil/v3 v3.23.10
	k8s.io/api v0.19.10
	k8s.io/apimachinery v0.19.10
	k8s.io/client-go v0.19.10
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.80.1
)

require (
	github.com/PuerkitoBio/purell v1.1.0 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/asaskevich/govalidator v0.0.0-20180720115003-f9ffefc3facf // indirect
	github.com/bytedance/sonic v1.9.1 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/globalsign/mgo v0.0.0-20180905125535-1ca0a4f7cbcb // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/analysis v0.17.0 // indirect
	github.com/go-openapi/errors v0.19.0 // indirect
	github.com/go-openapi/jsonpointer v0.17.0 // indirect
	github.com/go-openapi/jsonreference v0.17.0 // indirect
	github.com/go-openapi/loads v0.17.0 // indirect
	github.com/go-openapi/runtime v0.0.0-20180920151709-4f900dc2ade9 // indirect
	github.com/go-openapi/spec v0.17.0 // indirect
	github.com/go-openapi/strfmt v0.19.0 // indirect
	github.com/go-openapi/swag v0.19.0 // indirect
	github.com/go-openapi/validate v0.19.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.14.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/googleapis/gnostic v0.4.1 // indirect
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/imdario/mergo v0.3.5 // indirect
	github.com/klauspost/cpuid/v2 v2.2.4 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mailru/easyjson v0.0.0-20180823135443-60711f1a8329 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mitchellh/mapstructure v1.1.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.0.8 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	golang.org/x/arch v0.3.0 // indirect
	golang.org/x/crypto v0.9.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/oauth2 v0.0.0-20220223155221-ee480838109b // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/term v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/time v0.0.0-20220210224613-90d013bbcef8 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/utils v0.0.0-20221107191617-1a15be271d1d // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

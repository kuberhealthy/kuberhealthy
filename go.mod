module github.com/Comcast/kuberhealthy

replace github.com/go-resty/resty => gopkg.in/resty.v1 v1.10.0

replace google.golang.org/cloud => cloud.google.com/go v0.37.0

replace github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.3.0

require (
	github.com/Pallinder/go-randomdata v1.1.0
	github.com/aws/aws-sdk-go v1.25.24
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/denverdino/aliyungo v0.0.0-20191023002520-dba750c0c223 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-ini/ini v1.49.0 // indirect
	github.com/gogo/protobuf v1.3.0 // indirect
	github.com/golang/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/influxdata/influxdb1-client v0.0.0-20190402204710-8ff2fc3824fc
	github.com/integrii/flaggy v1.2.2
	github.com/pkg/errors v0.8.1
	github.com/pkg/sftp v1.10.1 // indirect
	github.com/sirupsen/logrus v1.4.0
	github.com/smartystreets/goconvey v1.6.4 // indirect
	golang.org/x/net v0.0.0-20190827160401-ba9fcec4b297 // indirect
	golang.org/x/sys v0.0.0-20190904005037-43c01164e931 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.51.0 // indirect
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190905160310-fb749d2f1064
	k8s.io/apimachinery v0.0.0-20190831074630-461753078381
	k8s.io/client-go v0.0.0-20190906195228-67a413f31aea
	k8s.io/klog v0.4.0
	k8s.io/kops v1.11.0
)

go 1.13

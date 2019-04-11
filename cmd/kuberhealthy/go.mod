module github.com/Comcast/kuberhealthy

replace github.com/go-resty/resty => gopkg.in/resty.v1 v1.10.0

replace google.golang.org/cloud => cloud.google.com/go v0.37.0

require (
	github.com/Comcast/kuberhealthy/pkg v0.0.0-20190326194825-fbf8476da15a
	github.com/Pallinder/go-randomdata v1.1.0
	github.com/integrii/flaggy v1.2.0
	github.com/sirupsen/logrus v1.4.0
	golang.org/x/net v0.0.0-20190326090315-15845e8f865b // indirect
	k8s.io/api v0.0.0-20190222213804-5cb15d344471 // indirect
	k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/klog v0.2.0 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)

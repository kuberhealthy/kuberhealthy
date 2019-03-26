module github.com/Comcast/kuberhealthy

replace github.com/go-resty/resty => gopkg.in/resty.v1 v1.10.0

replace google.golang.org/cloud => cloud.google.com/go v0.37.0

require (
	github.com/Comcast/kuberhealthy/pkg v0.0.0-20190326161711-72642aef9e4e
	github.com/Pallinder/go-randomdata v1.1.0
	github.com/integrii/flaggy v1.2.0
	github.com/sirupsen/logrus v1.4.0
	k8s.io/api v0.0.0-20181128191700-6db15a15d2d3
	k8s.io/apimachinery v0.0.0-20190118094746-1525e4dadd2d
	k8s.io/client-go v8.0.0+incompatible
)
